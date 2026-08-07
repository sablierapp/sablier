.PHONY: up down logs start

# Start all services
up:
	docker compose up -d

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs
logs:
	docker compose logs -f sablier

# Ask Sablier to start the whoami group (blocking strategy, 1 minute session)
start:
	@echo ">>> Requesting whoami group to start..."
	curl -sf "http://localhost:10000/api/strategies/blocking?group=whoami&session_duration=1m" | jq .
