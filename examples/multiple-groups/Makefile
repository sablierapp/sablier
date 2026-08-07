.PHONY: up down demo-team-a demo-team-b

up:
	docker compose up -d
	docker compose stop frontend backend shared-api

down:
	docker compose down

# Start team-a session: brings up frontend + shared-api.
demo-team-a: # up
	@echo "==> Requesting session for team-a (starts frontend + shared-api)..."
	time curl -s 'http://localhost:10000/api/strategies/blocking?group=team-a&timeout=30s'
	@echo ""
	@echo "==> Running containers:"
	docker compose ps --filter status=running

# Start team-b session: brings up backend + shared-api.
demo-team-b: # up
	@echo "==> Requesting session for team-b (starts backend + shared-api)..."
	time curl -s 'http://localhost:10000/api/strategies/blocking?group=team-b&timeout=30s'
	@echo ""
	@echo "==> Running containers:"
	docker compose ps --filter status=running
