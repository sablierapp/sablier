.PHONY: up down logs disconnect reconnect

# Start all services
up:
	docker compose up -d

# Stop and remove all services
down:
	docker compose down

# Follow sablier logs (shows reconnect attempts)
logs:
	docker compose logs -f sablier

# Simulate a Docker API connection loss by stopping the proxy
disconnect:
	@echo ">>> Stopping docker-proxy (simulating connection loss)..."
	docker compose stop docker-proxy

# Restore the proxy so sablier's reconnect loop can recover
reconnect:
	@echo ">>> Starting docker-proxy (restoring connection)..."
	docker compose start docker-proxy
