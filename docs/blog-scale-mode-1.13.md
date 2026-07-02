# Sablier 1.13: I Don't Want to Stop My Containers Anymore

*May 2026 · Alexis · [sablier.app](https://sablier.app)*

---

I've been running a Raspberry Pi 4 as my home server for a few years now. 4 GB of RAM,
a quad-core ARM Cortex-A72, and an SSD hanging off a USB 3.0 port. On paper it sounds
like a joke. In practice, it is one of the most satisfying pieces of hardware I own.

I love the constraint. There is something deeply satisfying about pushing a limited
machine to its absolute maximum — squeezing every last milliwatt out of it, making sure
nothing is wasted. I have probably ten services running on it at any given time: a
Gitea instance, a Calibre-Web library, a Vaultwarden password manager, Home Assistant,
a private Nextcloud, a few Forgejo runners… you get the picture. All self-hosted, all
on that one tiny board.

Sablier was born out of that same constraint. The idea is simple: if nobody is using a
service right now, why is it burning RAM and CPU? Stop it. Start it again the moment
someone knocks on the door. The first release was rough, but the concept worked. A lot
of you picked it up and ran with it.

But as the project has matured, and as I've lived with it on my own Pi for long enough,
I've noticed something about my actual usage patterns.

---

## Scale to Zero Isn't Always the Answer

The original pitch — stop your containers when idle — is still valid. For services I
access maybe once a week, stopping them completely makes perfect sense.

But there is a whole other category of services that I've started to think about
differently: the ones that are *always* doing *something*, even when I'm not actively
in front of them.

Take my Gitea instance. Even when I'm asleep, it is occasionally polling remotes,
sending notification emails, running background GC jobs. I don't need it to be *fast*
at 3 AM. I don't need it to respond in 50 ms when there is nobody there. But I do want
it to keep ticking along, quietly, in the background.

Stopping it entirely and starting it cold the moment I open a browser tab works, but
the cold start latency is noticeable — especially for heavier services like Nextcloud
or Home Assistant. And on a Pi, a fresh container start can take 10–20 seconds for
something with a real database behind it.

So I found myself stuck between two unsatisfying options:

1. Keep the service running full-blast 24/7, wasting the RAM and CPU I care about.
2. Stop it entirely and eat the cold-start penalty every time I want to use it.

What I actually wanted was a third option: **keep the service alive but throttled**, and
**restore full resources the instant I need it**.

---

## Introducing Scale Mode in Sablier 1.13

That third option is exactly what **Scale Mode** does in Sablier 1.13.

Instead of stopping a container when its session expires, Sablier uses `docker update`
(or the equivalent in Kubernetes / Swarm / Podman) to squash its CPU and memory limits
down to a minimal idle allocation. The process keeps running — it can still do its
background work — it just no longer gets to eat half your Pi's RAM while you're
sleeping.

The moment a new session is requested — the moment you open your browser, make an API
call, whatever — Sablier restores the original resource limits. No restart. No cold
start. The service is already warm. It just gets more headroom.

The whole thing is driven by six container labels:

```yaml
services:
  gitea:
    image: gitea/gitea:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=gitea"

      # --- Idle state (session expired) ---
      # Keep the container alive but throttle it
      - "sablier.idle.replicas=1"    # do NOT stop (replicas >= 1 enables scale mode)
      - "sablier.idle.cpu=0.1"       # 10 % of one core — enough to tick along
      - "sablier.idle.memory=128m"   # 128 MB — enough for a background GC pass

      # --- Active state (session requested) ---
      - "sablier.active.replicas=1"
      - "sablier.active.cpu=2.0"     # full 2 cores when I'm actually using it
      - "sablier.active.memory=512m" # 512 MB when I need it snappy
```

That's it. The `sablier.idle.replicas=1` label is the key: it tells Sablier "don't
stop this container, keep it alive at this replica count". The `cpu` and `memory` labels
are optional throttle knobs.

> **A quick note on the breaking change from earlier pre-releases:** if you were
> experimenting with the `sablier.idle.cpu` / `sablier.idle.memory` labels in an
> earlier preview build, you now also need `sablier.idle.replicas=1` explicitly.
> Setting only the CPU/memory labels without the replicas label no longer enables
> resource throttling — it is ignored. This was changed to make the intent
> unambiguous.

### How the transition looks in practice

Here is what happens step by step when scale mode is enabled:

```
[You close your browser tab]
    → Sablier's session timer starts

[Session expires after 5 minutes of inactivity]
    → docker update gitea --cpus 0.1 --memory 128m
    → container keeps running, background jobs keep ticking
    → RAM on the Pi drops from ~510 MB → ~130 MB for this service

[You open your browser tab again]
    → Sablier receives the request
    → docker update gitea --cpus 2.0 --memory 512m  (takes ~10ms)
    → your request hits a warm container
    → page loads in normal time — no cold start, no loading screen
```

On a Pi, going from `docker update` to a responsive container is essentially instant.
Compare that to a full cold start which can mean 15–20 seconds staring at the Sablier
waiting screen.

### Full working example

Here is a complete `compose.yml` for a stack using Sablier in scale mode, sitting
behind Traefik:

```yaml
services:
  sablier:
    image: sablierapp/sablier:1.13.0
    command:
      - start
      - --provider.name=docker
      - --sessions.default-duration=5m
      - --sessions.expiration-interval=10s
      - --server.metrics.enabled=true   # ← enable Prometheus metrics
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - "10000:10000"

  gitea:
    image: gitea/gitea:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=gitea"
      - "sablier.idle.replicas=1"
      - "sablier.idle.cpu=0.1"
      - "sablier.idle.memory=128m"
      - "sablier.active.replicas=1"
      - "sablier.active.cpu=2.0"
      - "sablier.active.memory=512m"
      # Traefik routing + Sablier middleware wiring
      - "traefik.enable=true"
      - "traefik.http.routers.gitea.rule=Host(`git.home.example.com`)"
      - "traefik.http.routers.gitea.middlewares=sablier@docker"
      - "traefik.http.middlewares.sablier.plugin.sablier.sablierUrl=http://sablier:10000"
      - "traefik.http.middlewares.sablier.plugin.sablier.group=gitea"
      - "traefik.http.middlewares.sablier.plugin.sablier.blocking.timeout=30s"
    volumes:
      - gitea-data:/data

volumes:
  gitea-data:
```

**Supported providers:** Docker, Docker Swarm, Kubernetes (Deployments and
StatefulSets), and Podman. Proxmox LXC doesn't support scale mode because its
provider uses tag-based config rather than key-value labels.

For Kubernetes, the same idea applies but through resource limit patches:

```yaml
# Kubernetes Deployment labels (under spec.template.metadata.labels)
sablier.enable: "true"
sablier.group: "gitea"
sablier.idle.replicas: "1"
sablier.idle.cpu: "100m"      # Kubernetes quantity notation
sablier.idle.memory: "128Mi"
sablier.active.replicas: "1"
sablier.active.cpu: "2000m"
sablier.active.memory: "512Mi"
```

---

## Measuring the Impact: Metrics Are Now Available

Here is the other thing I've been wanting for a long time: **knowing whether any of
this actually helps**.

Sablier 1.12 shipped a Prometheus-compatible `/metrics` endpoint
(`server.metrics.enabled: true`). If you are not already scraping it, you should be.
The key metric for measuring resource efficiency is:

```
sablier_instance_active_seconds_total{instance="<name>"}
```

This counter accumulates the total seconds each instance has spent in the **Ready**
(active, full-resource) state. You can use it to compute your **idle fraction** — the
percentage of time the container is running throttled or stopped, reclaiming resources:

```promql
# Fraction of the last 24 h that this instance was NOT actively serving traffic
1 - (
  increase(sablier_instance_active_seconds_total{instance="gitea"}[24h])
  / 86400
)
```

A result of `0.88` means the container was in its idle state 88 % of the day.

With scale mode, the meaning changes slightly: the container never stops, but for 88 %
of the day it is running at 10 % CPU and 128 MB instead of 2 cores and 512 MB. That is
still a massive win on a constrained machine.

---

## One Week of Data: Before vs. After

I've been running scale mode on my Pi for two weeks now, and the difference shows up
clearly in Grafana.

> 📸 **[Screenshot placeholder — insert your Grafana dashboard here]**
>
> *The dashboard shows `sablier_instance_active_seconds_total` for each managed
> service, plotted as a 24h rolling idle fraction. The left half of the graph
> (before scale mode) shows Gitea consuming ~480 MB of RAM 100 % of the time.
> The right half (after enabling scale mode) shows it flat at ~130 MB during
> off-hours, spiking to ~510 MB during the 2–3 hour windows when I'm actually
> using it.*

The setup to get that dashboard is straightforward. Add this to your Sablier config:

```yaml
server:
  metrics:
    enabled: true
```

Then scrape it with Prometheus:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: sablier
    static_configs:
      - targets: ["sablier:10000"]
    scrape_interval: 15s
```

A minimal Grafana panel to show the per-service active fraction over the last week:

```promql
# Panel: "Active fraction (last 7d)" — lower is better for idle services
increase(sablier_instance_active_seconds_total[7d]) / (7 * 86400)
```

I'll share the full Grafana dashboard JSON in the [examples/metrics](https://github.com/sablierapp/sablier/tree/main/examples/metrics) directory
once I have cleaned it up. If you put one together before me — please open a PR or drop
it in the discussions, I'll link it from the docs.

---

## Why This Matters More Than Stopping

Stopping a container frees 100 % of its RAM and CPU. Scale mode does not — you are
still holding `sablier.idle.memory` worth of RAM at all times. So why bother?

A few reasons that matter specifically in a constrained homelab environment:

**1. No cold-start penalty.**
On a Pi, a cold start for something like Gitea (Postgres, SSH daemon, the Gitea binary
itself) can mean 15–20 seconds of startup time. That is long enough to be annoying when
you just want to check a repo. Scale mode eliminates this entirely.

**2. Background jobs keep running.**
Some services have cron-like background tasks — mirror syncing, email notifications,
index updates. Stopping the container kills those. Scale mode lets them keep running,
just slowly, which is usually fine for background work.

**3. Connection state is preserved.**
Database connections, open file handles, in-memory caches — stopping a container
throws all of this away. Throttled containers keep their state. A Gitea instance that
was serving you 3 hours ago still has its hot caches when you come back.

**4. It composes with other constraints.**
I run several services at `sablier.idle.cpu=0.1`. At 10 % of one core, multiple idle
services can coexist happily on the same Pi alongside the services that are actually
awake. The Pi's four cores can easily park six or seven throttled-but-alive services
while two or three are actively serving requests.

---

## What Are You Going to Build With This?

I wrote scale mode because it solved my own problem. But the moment I started thinking
about it more broadly, I realised there are a lot of interesting ways to use it that I
haven't thought of yet.

A few questions I'm genuinely curious about:

- **Batch workloads:** Could you use scale mode to throttle a heavy background
  processing service (video transcoding, ML inference) down to minimal resources during
  peak homelab hours, then let it rip overnight?

- **Cost optimization on cloud:** Could you use this with Kubernetes to keep pods
  alive but resource-minimal during off-peak hours, avoiding the scheduling delay of a
  fresh pod, while still saving on resource quotas?

- **Multi-tier scaling:** `sablier.active.replicas` can be set higher than
  `sablier.idle.replicas`. Could you build a system that auto-scales from 1 idle
  replica to 3 active replicas on demand, with full resource restoration, all driven by
  a single traffic hit?

I'd love to see what people come up with. Open a discussion on GitHub, drop by the
subreddit, or just tag me if you write something up. The [examples/scale-mode](https://github.com/sablierapp/sablier/tree/main/examples/scale-mode)
directory has a minimal working stack you can clone and adapt.

---

## What's Next: Disk Resources and Anti-Affinities

Scale mode covers CPU and memory. Those are the obvious knobs. But living on a Pi long
enough teaches you that there is a third resource that matters just as much, and that
nobody talks about: **disk I/O**.

My Pi has an SSD on USB 3.0. Fast enough for normal homelab work, but the moment two
services compete for it simultaneously, the whole thing becomes sluggish. The worst
offender is my media server. Let me be upfront: you are not transcoding 4K on a Pi 4.
That is simply not a thing that happens. What I do is direct stream through Plex — the
file goes from disk to the network client untouched, no CPU involved. That works
perfectly fine, right up until something else starts hammering the SSD at the same
time.

The culprit is usually my Nextcloud background job scanning and hashing a newly synced
folder, or a Gitea mirror sync pulling a large repo. A high-bitrate 4K Remux at
80–100 Mbps needs consistent, uninterrupted disk reads. The moment another process
starts competing for that USB 3.0 bandwidth, the read throughput drops, the Plex
buffer can't keep ahead of playback, and the stream stutters or outright drops. Direct
stream has no fallback — there is no transcoded lower-bitrate version to fall back to.
You either have the disk, or the stream dies.

### Disk I/O scheduling with `blkio-weight` — available now in 1.13

I shipped this one too. It's already in 1.13.

Rather than hard rate limits (which require knowing the block device path), Sablier
uses the kernel's I/O scheduler **weight** — a relative priority between 10 and 1000.
A container with weight 800 gets roughly 8× the I/O scheduling slots of one at 100
when they both compete for the same disk. When there is no contention, both run at
full speed.

Same idle/active label pattern as CPU and memory:

```yaml
services:
  nextcloud:
    image: nextcloud:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=nextcloud"
      - "sablier.idle.replicas=1"
      - "sablier.idle.cpu=0.1"
      - "sablier.idle.memory=128m"
      - "sablier.idle.blkio-weight=50"    # lowest priority I/O when idle
      - "sablier.active.replicas=1"
      - "sablier.active.cpu=2.0"
      - "sablier.active.memory=512m"
      - "sablier.active.blkio-weight=500" # default weight when actively used
```

When Nextcloud's session expires and nobody is using it, Sablier calls `docker update`
with `--blkio-weight 50`. When a session is requested, it restores to `500`. The Plex
container isn't managed by Sablier at all — it just runs at the kernel default weight
of 500 continuously. Because idle Nextcloud is at 50, Plex gets roughly 10× more I/O
scheduling priority when both are reading the disk at the same time.

**Valid range:** 10–1000. The label is silently ignored if the value is out of range or
malformed. Zero (or an unset label) means no change to the container's current weight.

**Supported on:** Docker and Podman. Docker Swarm's API does not expose blkio weight at
the service level, and Kubernetes resource limits do not yet include I/O weight — both
are skipped silently.

> **Raspberry Pi OS note:** Raspberry Pi OS Bookworm uses cgroup v2 by default. Docker
> on cgroup v2 maps `blkio-weight` to `io.weight` (range 1–10000), scaled automatically
> from the 10–1000 Docker API range. It works — just make sure `cgroup_enable=memory
> swapaccount=1` is in your `/boot/firmware/cmdline.txt` for full cgroup v2 support.

### Anti-affinities: teaching Sablier about conflicts

Blkio weight helps each service define its own I/O priority — but it does not let
services coordinate with each other. Nextcloud does not know that Plex is streaming
right now. It just runs at weight 50 because its session happened to expire.

Anti-affinities let you declare that relationship explicitly:

```yaml
services:
  # Plex: declare itself as part of the "streaming" group.
  plex:
    image: plexinc/pms-docker:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=streaming"

  # Nextcloud: back off whenever the "streaming" group is active.
  nextcloud:
    image: nextcloud:latest
    labels:
      - "sablier.enable=true"
      - "sablier.group=nextcloud"
      - "sablier.idle.replicas=1"
      - "sablier.idle.cpu=0.1"
      - "sablier.idle.memory=128m"
      - "sablier.idle.blkio-weight=50"
      - "sablier.active.replicas=1"
      - "sablier.active.cpu=2.0"
      - "sablier.active.memory=512m"
      - "sablier.active.blkio-weight=500"
      # Anti-affinity: apply idle constraints the instant "streaming" becomes active.
      - "sablier.anti-affinity=streaming"
```

When a session for the `streaming` group becomes active, Sablier automatically applies
idle constraints to every service that declared an anti-affinity against it — regardless
of whether those services have their own active session. Nextcloud gets throttled the
moment someone hits Play, not sometime later when its own session eventually expires.

And it goes further than that. While the streaming session is active, **Nextcloud cannot
be started**. Any request to wake Nextcloud up returns `status=anti-affinity-blocked`:

```json
{
  "instances": [{ "instance": { "name": "nextcloud", "status": "anti-affinity-blocked",
                                "message": "blocked by active group \"streaming\"" } }],
  "status": "not-ready"
}
```

The block lifts automatically the moment the streaming session expires. This is the key
difference from a plain idle priority: it is not just "Nextcloud gets fewer resources
while Plex streams", it is "Nextcloud **cannot compete** with Plex at all while Plex is
active". Your stream gets the disk uncontested.

This is a fundamentally different control axis from what existed before. Previously,
Sablier reacted to *whether a service is being used*. Anti-affinities let it react to
*what else is being used at the same time*. That is closer to how I actually think about
my homelab: I don't just want services to be efficient in isolation, I want them to be
*good neighbours* to each other.

When the streaming session expires, Nextcloud is **not** automatically restored to full
resources — it stays in its throttled idle state until someone actually requests a
Nextcloud session. That is the right behaviour: there is no point waking Nextcloud up
just because Plex stopped.

A comma-separated list is supported for multi-group conflicts:

```yaml
- "sablier.anti-affinity=streaming,gaming"
```

Anti-affinities also compose with the stop behaviour. If Nextcloud had
`sablier.idle.replicas=0` (the default), Sablier would stop the container entirely
when a streaming session starts, rather than throttling it. For something like a
background job runner that should not compete with a time-sensitive stream at all,
stopping is the right call.

The [examples/anti-affinity](https://github.com/sablierapp/sablier/tree/main/examples/anti-affinity)
directory has a minimal working stack.

---

## Getting Started

Sablier 1.13.0 is available now:

```bash
docker pull sablierapp/sablier:1.13.0
```

The [full documentation for scale mode](https://sablier.app/configuration#scale-mode)
covers all the label combinations, provider-specific notes (Docker swap behaviour,
Kubernetes resource limits vs. requests, Swarm task re-scheduling), and the migration
path from earlier preview builds.

If you are already running Sablier with the classic stop/start behaviour, scale mode is
purely additive — you opt in per-service by adding the `sablier.idle.replicas` label.
Everything else stays the same.

---

## Closing Thoughts

I didn't set out to build a resource throttling system. I set out to make my Pi not run
out of RAM. Scale mode is what happens when you actually live with your own software
long enough to find its rough edges.

If you are a selfhosted enthusiast running on constrained hardware — a Pi, an old
laptop, a cheap VPS — I think scale mode changes the calculus significantly. You don't
have to choose between "dead and cold" or "alive and expensive". There is a third option
now, and it turns out to be the one I actually wanted all along.

Enable metrics, set up Grafana, and let me know what your idle fraction looks like
after a week. I'm genuinely curious.

---

*Found a bug? Have feedback? Open an issue at [github.com/sablierapp/sablier](https://github.com/sablierapp/sablier).*
*Want to say hi? I'm on Reddit as [u/sablierapp](https://www.reddit.com/user/sablierapp).*
