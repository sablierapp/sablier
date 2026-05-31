.PHONY: up down logs start ps

# Start sablier, then *create* (but don't start) the dependency graph so the
# whole chain (db → migration → app) is scaled to zero and triggered
# on-demand by Sablier.
#
# We deliberately do NOT `docker compose up` the managed services: because they
# carry sablier.enable=true, Sablier would immediately stop them again (it
# scales to zero anything it didn't start), which races with Compose's native
# depends_on health-wait. Creating them leaves them ready to be started by
# Sablier on the first request.
up:
	docker compose up -d sablier
	docker compose create app migration db

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
