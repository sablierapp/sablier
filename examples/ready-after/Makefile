.PHONY: up down logs start

# Start all services (slow-starter is initially running; stop it first to demo the flow)
up:
	docker compose up -d
	docker compose stop slow-starter

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs
logs:
	docker compose logs -f sablier

# Ask Sablier to start the slow-starter group (blocking — waits until ready-after elapses)
start:
	@echo ">>> Requesting slow-starter group (blocking, up to 2m timeout)..."
	curl -sf "http://localhost:10000/api/strategies/blocking?names=slow-starter&session_duration=2m&timeout=2m" | jq .
