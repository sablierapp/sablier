# Activity Tracking Example

This example demonstrates Sablier's **activity tracking**: traffic on a container's ports keeps its session open, even when no request goes through a reverse proxy.

Sablier counts the bytes on the watched container's ports with eBPF. Every `--activity.interval`, a container whose traffic reaches its `sablier.activity.min-bytes` opens or extends the session of each of its groups, like a proxy request would. Traffic from containers labeled `sablier.activity.ignore-source=true` never counts.

## Requirements

- A Linux Docker host on amd64 or arm64, with cgroup v2 and kernel 5.8 or newer.
- A Sablier image with activity tracking. It is newer than the latest release: until it ships, run `SABLIER_IMAGE=sablierapp/sablier:next make demo`.

Sablier needs the host cgroup namespace and two capabilities. On a Raspberry Pi 4 with Docker 29.7.1 and kernel 6.8, nothing else was needed; before Linux 5.11, also add `SYS_RESOURCE`.

```yaml
sablier:
  cgroup: host
  cap_add:
    - BPF
    - NET_ADMIN
```

## This Example

| Service | Labels | Behaviour |
|---------|--------|-----------|
| `stream` | `sablier.group=stream`, `sablier.activity.enable=true`, `sablier.activity.session-duration=30s` | An nginx serving a 200 MB file. Traffic on its exposed port, 80, extends the `stream` session by 30s. |
| `poller` | `sablier.activity.ignore-source=true` | Fetches 1 MiB from `stream` every 2s, far more than `min-bytes`. Sablier ignores it, so it never keeps `stream` running. |
| `downloader` | none | The demo's client. It downloads the file at 200 KB/s for 90s, and its traffic counts. |

Sablier reads the counters every 5s (`--activity.interval=5s`) and exposes `/metrics`, where the demo reads the session's expiry and the bytes counted on `stream`. Reading `/metrics` never renews a session.

## Running the Example

```bash
# Start Sablier, stream and the poller
make up

# Open a 30s session, download for 90s, then watch the session expire
make demo

# Tear down
make down
```

`make demo`:

1. opens a 30s session for the `stream` group through `/api/strategies/blocking`;
2. runs `downloader`: `curl --limit-rate 200k` for 90s;
3. prints the session's remaining time every 10s while it downloads;
4. prints it every 5s once the download stops, until the session expires.

The poller runs the whole time.

## What You'll See

Output from a Raspberry Pi 4 (Docker 29.9.2, kernel 6.8.0-1065-raspi). Times are UTC.

```text
==> Opening a 30s session for the stream group...
04:39:14  session: 30s left  counted on stream:   0.0 MB
==> Downloading from stream for 90s at 200 KB/s...
04:39:24  session: 28s left  counted on stream:   3.1 MB
04:39:34  session: 28s left  counted on stream:   5.2 MB
04:39:44  session: 28s left  counted on stream:   7.3 MB
04:39:54  session: 28s left  counted on stream:   9.3 MB
04:40:04  session: 28s left  counted on stream:  11.3 MB
04:40:14  session: 28s left  counted on stream:  13.4 MB
04:40:24  session: 28s left  counted on stream:  15.5 MB
04:40:34  session: 28s left  counted on stream:  17.5 MB
==> Download stopped at 04:40:46. Only the poller is left:
04:40:51  session: 26s left  counted on stream:  20.3 MB
04:40:56  session: 21s left  counted on stream:  20.3 MB
04:41:01  session: 16s left  counted on stream:  20.3 MB
04:41:07  session: 10s left  counted on stream:  20.3 MB
04:41:12  session: 5s left   counted on stream:  20.3 MB
04:41:17  session: expired   counted on stream:  20.3 MB
==> Session expired 31s after the download stopped.
    Meanwhile the poller fetched 15.7 MB from stream in 15 requests.
poller: running
sablier: running
stream: exited
```

- **While the download runs, the session never drops below 28s.** Each 5s interval carries about 1 MB, so every interval refreshes the session to 30s.
- **The session expires 31s after the download stops:** 30s after the last interval that counted traffic, the tail of the download.
- **The poller never extends it.** It fetched 15.7 MB after the download stopped, and the bytes counted on `stream` stayed at 20.3 MB.

## Through a Published Port

`make demo FROM=host` downloads from the host through `stream`'s published port, `localhost:8080`, instead of from the `downloader` container. The same run on the same Pi:

```text
==> Downloading from stream for 90s at 200 KB/s...
04:36:32  session: 24s left  counted on stream:  19.1 MB
04:36:42  session: 24s left  counted on stream:  22.0 MB
04:36:52  session: 29s left  counted on stream:  23.0 MB
04:37:02  session: 24s left  counted on stream:  25.8 MB
04:37:12  session: 29s left  counted on stream:  43.6 MB
04:37:23  session: 18s left  counted on stream:  43.6 MB
04:37:33  session: 28s left  counted on stream:  47.2 MB
04:37:43  session: 18s left  counted on stream:  47.2 MB
==> Download stopped at 04:37:52. Only the poller is left:
04:37:58  session: 23s left  counted on stream:  51.2 MB
04:38:03  session: 18s left  counted on stream:  51.2 MB
04:38:08  session: 13s left  counted on stream:  51.2 MB
04:38:13  session: 8s left   counted on stream:  51.2 MB
04:38:19  session: 2s left   counted on stream:  51.2 MB
==> Session expired 29s after the download stopped.
    Meanwhile the poller fetched 14.7 MB from stream in 14 requests.
```

The client still reads 200 KB/s, but `stream` sends in bursts. From the host, a published port goes through Docker's proxy, which reads the response ahead of the client: Sablier counted 19.1 MB in the first 10s, then the counter stood still for 10s or more at a time. Sablier counts what crosses the container's cgroup, so the intervals without traffic don't refresh the session, and the remaining time dips to 18s. The session still never expires, because the gaps between bursts are shorter than the 30s session duration.

Media players buffer the same way, so give `sablier.activity.session-duration` a margin over the longest gap between bursts. The [feature page](https://sablierapp.dev/features/activity/) has the numbers measured on Plex.

## Manual Steps

```bash
docker compose up -d

# 1. Open a 30s session
curl 'http://localhost:10000/api/strategies/blocking?group=stream&session_duration=30s&timeout=60s'

# 2. Download for 90s from another container
docker compose run --rm -T downloader curl -s -o /dev/null --limit-rate 200k --max-time 90 http://stream/video.bin

# Meanwhile, in another terminal: the session's expiry (Unix time) and the bytes counted
curl -s http://localhost:10000/metrics | grep -E '^sablier_(session_expires_at_timestamp_seconds|activity_bytes_total)'

# 3. Watch the poller's fetches: 206 responses of 1048576 bytes every 2s
docker compose logs -f poller
```

## Notes

- **Activity never starts a stopped container.** Once the session expires, Sablier stops `stream`, and the next proxy or API request starts it again.
- **Sablier always ignores its own addresses and loopback.** Docker health checks run inside the container and reach the service over loopback, so they never count. In this example Sablier has its own container, so traffic from the host or from other containers counts.
- **Label the entry point, not its dependencies.** Traffic on `stream` extends every member of its groups.
- **Pick `sablier.activity.min-bytes` from `rate(sablier_activity_bytes_total[5m])`:** above what the container carries while idle, below what it carries in use.

📚 **[Full documentation](https://sablierapp.dev/features/activity/)**
