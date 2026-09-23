.PHONY: up down logs start status

# Start Nomad and Sablier, then register the whoami job with count 0.
up:
	docker compose up -d --wait
	docker compose exec nomad nomad job run -detach /jobs/whoami.nomad.hcl

# Stop and remove all services.
down:
	docker compose down

# Follow the Sablier logs.
logs:
	docker compose logs -f sablier

# Ask Sablier for whoami/web (blocking strategy, 1 minute session).
start:
	@echo ">>> Requesting whoami/web (blocking, 1m session)..."
	time curl -sf "http://localhost:10000/api/strategies/blocking?names=whoami/web&session_duration=1m" | jq .

# Show the count and the allocations of the whoami job.
status:
	docker compose exec nomad nomad job status whoami
