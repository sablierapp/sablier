.PHONY: up down logs start ps

# Start sablier, then create the dependency graph in a stopped state so the
# whole chain (db → migration → app) is triggered on-demand by Sablier.
up:
	docker compose up -d
	docker compose stop app migration db

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs
logs:
	docker compose logs -f sablier

# Show the state of every container in the stack
ps:
	docker compose ps -a

# Ask Sablier to start the `app` group (blocking — resolves depends_on first)
start:
	@echo ">>> Requesting app group (blocking, up to 2m timeout)..."
	curl -v -sf "http://localhost:10000/api/strategies/blocking?group=app&session_duration=2m&timeout=2m" | jq .
