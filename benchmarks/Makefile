COMPOSE  = docker compose -f compose.yml
BOMB     = go tool -modfile=../tools.mod bombardier
SABLIER  = http://localhost:10000

# Blocking strategy — waits until the target is ready before responding.
BLOCKING_WARM_URL = "$(SABLIER)/api/strategies/blocking?group=benchmark&session_duration=1h&timeout=30s"
BLOCKING_COLD_URL = "$(SABLIER)/api/strategies/blocking?group=benchmark&session_duration=1h&timeout=120s"

# Dynamic strategy — returns immediately with the current readiness state.
DYNAMIC_WARM_URL  = "$(SABLIER)/api/strategies/dynamic?group=benchmark&session_duration=1h"

.PHONY: up down bench bench-blocking-warm bench-blocking-cold bench-dynamic-warm bench-dynamic-cold clean

## Pull images and start the stack (Sablier + target).
up:
	$(COMPOSE) up -d

## Stop and remove all containers.
down:
	$(COMPOSE) down --remove-orphans

## Run all four benchmark scenarios in sequence.
bench: bench-blocking-warm bench-blocking-cold bench-dynamic-warm bench-dynamic-cold

## Blocking warm — measures Sablier's per-request overhead when the target is
## already running and the session cache is hot.  Every measured request hits
## only Sablier's routing and session-lookup code path.
bench-blocking-warm: up
	@echo "=== blocking / warm ==="
	@echo "Priming session cache..."
	@curl -sf $(BLOCKING_WARM_URL) > /dev/null
	@echo "Running (10 connections, 30 s)..."
	$(BOMB) -c 10 -d 30s -l $(BLOCKING_WARM_URL)

## Blocking cold — measures the full round-trip including Docker start time.
## The target is stopped before each run so every request triggers a cold start.
## Set N to control the number of samples (default 3).
bench-blocking-cold: up
	@echo "=== blocking / cold ==="
	@echo "Stopping target for cold-start measurement..."
	$(COMPOSE) stop target
	@echo "Running (1 connection, $(or $(N),3) requests)..."
	$(BOMB) -c 1 -n $(or $(N),3) -l --timeout 120s $(BLOCKING_COLD_URL)
	$(COMPOSE) start target

## Dynamic warm — measures the overhead of the dynamic endpoint when the
## target is already running.  The endpoint returns immediately (no waiting),
## so this isolates Sablier's response-rendering cost.
bench-dynamic-warm: up
	@echo "=== dynamic / warm ==="
	@echo "Priming session cache..."
	@curl -sf $(BLOCKING_WARM_URL) > /dev/null
	@echo "Running (10 connections, 30 s)..."
	$(BOMB) -c 10 -d 30s -l $(DYNAMIC_WARM_URL)

## Dynamic cold — measures the overhead of the dynamic endpoint when the
## target is stopped.  The endpoint still returns immediately with a
## not-ready response, showing the cost of the not-ready rendering path.
bench-dynamic-cold: up
	@echo "=== dynamic / cold ==="
	@echo "Stopping target..."
	$(COMPOSE) stop target
	@echo "Running (10 connections, 30 s)..."
	$(BOMB) -c 10 -d 30s -l $(DYNAMIC_WARM_URL)
	$(COMPOSE) start target

## Remove containers and all pulled images.
clean: down
	$(COMPOSE) down --rmi local
