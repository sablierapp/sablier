.PHONY: up down demo

SABLIER := http://localhost:10000

# `make demo` downloads from the downloader container, on the compose network.
# `make demo FROM=host` downloads from this host through stream's published
# port instead, which makes the traffic bursty (see the README).
CURL := curl -s -o /dev/null --limit-rate 200k --max-time 90
ifeq ($(FROM),host)
DOWNLOAD := $(CURL) http://localhost:8080/video.bin
else
DOWNLOAD := docker compose run --rm -T downloader $(CURL) http://stream/video.bin 2>&1 | grep -Ev '^( Container |$$)'
endif

# Prints the stream session's remaining time and the bytes Sablier counted on
# stream, both read from /metrics. Reading /metrics never renews a session.
STATUS = curl -s $(SABLIER)/metrics | awk -v now=$$(date +%s) -v t=$$(date -u +%T) ' \
	/^sablier_session_expires_at_timestamp_seconds\{group="stream"/ { left = sprintf("%ds left", $$2 - now) } \
	/^sablier_activity_bytes_total\{instance="[^"]*stream/ { bytes = $$2 } \
	END { printf "%s  session: %-8s  counted on stream: %5.1f MB\n", t, (left == "" ? "expired" : left), bytes / 1e6 }'

up:
	docker compose up -d

down:
	docker compose down

# Demo: open a 30s session for the stream group, download from stream for 90s,
# then stop. The download keeps the session open; the poller, which runs the
# whole time, does not. The session expires about 30s after the download stops.
demo: up
	@echo "==> Waiting for Sablier..."
	@for i in $$(seq 30); do curl -sf $(SABLIER)/health > /dev/null && exit 0; sleep 1; done; \
	docker compose logs sablier; exit 1
	@echo "==> Opening a 30s session for the stream group..."
	@curl -sf '$(SABLIER)/api/strategies/blocking?group=stream&session_duration=30s&timeout=60s' > /dev/null
	@$(STATUS)
	@echo "==> Downloading from stream for 90s at 200 KB/s..."
	@( for i in 1 2 3 4 5 6 7 8; do sleep 10; $(STATUS); done ) & \
	$(DOWNLOAD); \
	stopped=$$(date +%s); wait; \
	echo "==> Download stopped at $$(date -u +%T). Only the poller is left:"; \
	n=0; while curl -s $(SABLIER)/metrics | grep -q '^sablier_session_expires_at_timestamp_seconds{group="stream"'; do \
		sleep 1; n=$$((n + 1)); if [ $$((n % 5)) -eq 0 ]; then $(STATUS); fi; \
	done; \
	echo "==> Session expired $$(( $$(date +%s) - stopped ))s after the download stopped."; \
	docker compose logs --no-log-prefix poller | awk -v since=$$stopped \
		'$$1 >= since && $$2 == 206 { n++; b += $$3 } END { printf "    Meanwhile the poller fetched %.1f MB from stream in %d requests.\n", b / 1e6, n }'
	@sleep 3
	@docker compose ps -a --format '{{.Service}}: {{.State}}'
