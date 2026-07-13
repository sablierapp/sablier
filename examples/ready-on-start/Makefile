.PHONY: up down logs start

# Start all services (both containers are stopped initially)
up:
	docker compose up -d
	docker compose stop web nvr

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs
logs:
	docker compose logs -f sablier

# Ask Sablier to start the home group (blocking — nvr passes through, web waits for health)
start:
	@echo ">>> Requesting home group (blocking)..."
	time curl -sf "http://localhost:10000/api/strategies/blocking?group=home&session_duration=2m" | jq .
