.PHONY: up down logs ps request-dynamic request-blocking stop-target traces

up:
	docker compose up -d --remove-orphans
	@echo
	@echo "Sablier:    http://localhost:10000"
	@echo "  /health   http://localhost:10000/health"
	@echo "Jaeger UI:  http://localhost:16686"
	@echo
	@echo "Run 'make request-blocking' to trigger a request and generate traces."

down:
	docker compose down -v --remove-orphans

logs:
	docker compose logs -f sablier

ps:
	docker compose ps

# Dynamic strategy: returns an HTML loading page immediately.
request-dynamic:
	@curl -s -o /dev/null -w "HTTP %{http_code}\n" \
	  'http://localhost:10000/api/strategies/dynamic?group=demo&session_duration=1m'

# Blocking strategy: waits until the "demo" group is ready.
request-blocking:
	@curl -s 'http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m'

# Manually stop the target to observe a cold start on the next request.
stop-target:
	docker compose stop whoami

# Open the Jaeger UI to inspect traces (macOS).
traces:
	open http://localhost:16686/search?service=sablier
