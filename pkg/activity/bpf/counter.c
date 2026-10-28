//go:build ignore

// counter counts the bytes a watched cgroup sends and receives on its watched
// ports. It is loaded once per watched cgroup with the constant `slot`
// rewritten, attached to that cgroup's inet ingress and egress hooks, and
// shares the three maps below with every other loaded copy.
//
// Under cgroup_skb, offset 0 of the packet is the IP header and skb->len
// counts from there.

#include <linux/bpf.h>
#include <linux/in.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define MAX_WATCHED 256

// The fragment offset bits of iphdr.frag_off (include/net/ip.h, not UAPI).
#define IP_FRAG_OFFSET 0x1fff

// The slot this copy of the program counts into; set at load time.
const volatile __u32 slot = 0;

struct port_key {
	__u32 slot;
	__u16 port; // host byte order
	__u8 proto;
	__u8 pad;
};

struct lpm_key {
	__u32 prefixlen;
	__u8 addr[16]; // IPv6, or IPv4-mapped IPv6 (::ffff:a.b.c.d)
};

// {slot, port, proto} for every watched port of every slot.
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 4096);
	__type(key, struct port_key);
	__type(value, __u8);
} ports SEC(".maps");

// Remote addresses whose traffic is never counted.
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 1024);
	__uint(map_flags, BPF_F_NO_PREALLOC);
	__type(key, struct lpm_key);
	__type(value, __u8);
} ignored SEC(".maps");

// Running byte total per slot.
struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, MAX_WATCHED);
	__type(key, __u32);
	__type(value, __u64);
} bytes SEC(".maps");

static __always_inline void count(struct __sk_buff *skb, int ingress)
{
	struct lpm_key remote = {.prefixlen = 128};
	struct port_key key = {.slot = slot};
	__be16 l4ports[2]; // source, destination
	__u32 l4off;
	__u8 version;

	if (bpf_skb_load_bytes(skb, 0, &version, sizeof(version)) < 0)
		return;
	version >>= 4;

	if (version == 4) {
		struct iphdr ip;

		if (bpf_skb_load_bytes(skb, 0, &ip, sizeof(ip)) < 0)
			return;
		if (ip.frag_off & bpf_htons(IP_FRAG_OFFSET))
			return;
		if (ip.ihl < 5)
			return;
		key.proto = ip.protocol;
		l4off = ip.ihl * 4;
		remote.addr[10] = 0xff;
		remote.addr[11] = 0xff;
		__builtin_memcpy(&remote.addr[12], ingress ? &ip.saddr : &ip.daddr, 4);
	} else if (version == 6) {
		struct ipv6hdr ip6;

		if (bpf_skb_load_bytes(skb, 0, &ip6, sizeof(ip6)) < 0)
			return;
		// Extension headers, fragment headers included, are not followed.
		key.proto = ip6.nexthdr;
		l4off = sizeof(ip6);
		__builtin_memcpy(remote.addr, ingress ? &ip6.saddr : &ip6.daddr, 16);
	} else {
		return;
	}

	if (key.proto != IPPROTO_TCP && key.proto != IPPROTO_UDP)
		return;
	// TCP and UDP both start with the source and destination ports.
	if (bpf_skb_load_bytes(skb, l4off, l4ports, sizeof(l4ports)) < 0)
		return;
	key.port = bpf_ntohs(ingress ? l4ports[1] : l4ports[0]);

	if (!bpf_map_lookup_elem(&ports, &key))
		return;
	if (bpf_map_lookup_elem(&ignored, &remote))
		return;

	__u32 s = slot;
	__u64 *total = bpf_map_lookup_elem(&bytes, &s);
	if (total)
		// Atomic: runs on one CPU can interleave (softirq, preemption).
		__sync_fetch_and_add(total, skb->len);
}

SEC("cgroup_skb/ingress")
int count_ingress(struct __sk_buff *skb)
{
	count(skb, 1);
	return 1;
}

SEC("cgroup_skb/egress")
int count_egress(struct __sk_buff *skb)
{
	count(skb, 0);
	return 1;
}

char __license[] SEC("license") = "Dual MIT/GPL";
