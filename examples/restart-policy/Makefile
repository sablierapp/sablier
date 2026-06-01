.PHONY: up down logs start ps

# Start sablier, then *create* (but don't start) the group members so they are
# scaled to zero and triggered on-demand by Sablier. We deliberately do NOT
# `docker compose up` the managed services: because they carry
# sablier.enable=true, Sablier would immediately stop them again (it scales to
# zero anything it didn't start).
up:
	docker compose up -d sablier
	docker compose create app init

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs
logs:
	docker compose logs -f sablier

# Show the state of every container in the stack
ps:
	docker compose ps -a

# Ask Sablier to start the `demo` group (blocking). The group only becomes
# ready once `app` is running AND `init` has completed. This succeeds because
# --provider.docker.honor-restart-policy=true reports the exited `init`
# container as ready.
start:
	@echo ">>> Requesting demo group (blocking, up to 2m timeout)..."
	curl -v -sf "http://localhost:10000/api/strategies/blocking?group=demo&session_duration=2m&timeout=2m" | jq .
