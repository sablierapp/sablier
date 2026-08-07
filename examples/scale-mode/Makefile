.PHONY: up down demo

up:
	docker compose up -d

down:
	docker compose down

# Demo: request the whoami service (starts scale-mode if idle), wait, then see it throttled
demo: up
	@echo "==> Requesting session (applies active resources)..."
	curl -s 'http://localhost:10000/api/strategies/blocking?group=whoami&timeout=30s' | head -5
	@echo ""
	@echo "==> Container resources after activation:"
	docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores, {{.HostConfig.Memory}} bytes memory' 2>/dev/null || \
		docker inspect $$(docker compose ps -q whoami) --format '{{.HostConfig.NanoCPUs}} nanocores, {{.HostConfig.Memory}} bytes memory'
	@echo ""
	@echo "==> Waiting 40s for session to expire..."
	sleep 40
	@echo "==> Container resources after idle (should be throttled):"
	docker inspect scale-mode-whoami-1 --format '{{.HostConfig.NanoCPUs}} nanocores, {{.HostConfig.Memory}} bytes memory' 2>/dev/null || \
		docker inspect $$(docker compose ps -q whoami) --format '{{.HostConfig.NanoCPUs}} nanocores, {{.HostConfig.Memory}} bytes memory'
