.PHONY: up down logs ps metrics request-dynamic request-blocking stop-target

up:
	docker compose up -d --remove-orphans
	@echo
	@echo "Sablier:    http://localhost:10000"
	@echo "  /health   http://localhost:10000/health"
	@echo "  /metrics  http://localhost:10000/metrics"
	@echo "Prometheus: http://localhost:9090"

down:
	docker compose down -v --remove-orphans

logs:
	docker compose logs -f sablier

ps:
	docker compose ps

# Print only Sablier-emitted metrics (skip the standard go_/process_ collectors).
metrics:
	@curl -s http://localhost:10000/metrics | grep -E '^sablier_'

# Dynamic strategy: returns an HTML loading page immediately, schedules an async
# start of the "demo" group. After the first call, sablier_group_locked{group="demo"}
# becomes 1 and sablier_session_requests_total{strategy="dynamic"} increments.
request-dynamic:
	@curl -s -o /dev/null -w "HTTP %{http_code}\n" \
	  'http://localhost:10000/api/strategies/dynamic?group=demo&session_duration=1m'

# Blocking strategy: waits up to default-timeout for the group to become Ready.
# Populates sablier_instance_start_duration_seconds and
# sablier_instance_ready_duration_seconds histograms.
request-blocking:
	@curl -s 'http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m'

# Manually stop the target outside of Sablier so the next request triggers a
# cold start that you can observe in the warmup histograms.
stop-target:
	docker compose stop whoami
