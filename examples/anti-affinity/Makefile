.PHONY: up down demo

up:
	docker compose up -d

down:
	docker compose down

# Demo: bring the background service up, then request streaming and watch the
# background service get forced idle (stopped) while streaming is active — and
# restored once the streaming session expires.
demo: up
	@echo "==> Activating the background service..."
	curl -s 'http://localhost:10000/api/strategies/blocking?group=background&timeout=30s' > /dev/null
	@echo "==> State after activation (background should be running):"
	@docker compose ps --format '{{.Service}}: {{.State}}'
	@echo ""
	@echo "==> Requesting the streaming service (background must yield)..."
	curl -s 'http://localhost:10000/api/strategies/blocking?group=streaming&timeout=30s' > /dev/null
	@sleep 3
	@echo "==> State while streaming is active (background should be exited):"
	@docker compose ps -a --format '{{.Service}}: {{.State}}'
	@echo ""
	@echo "==> Waiting 40s for the streaming session to expire..."
	@sleep 40
	@echo "==> State after streaming expired (background should be running again):"
	@docker compose ps --format '{{.Service}}: {{.State}}'
