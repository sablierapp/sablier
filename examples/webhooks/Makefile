.DEFAULT_GOAL := help

COMPOSE := docker compose

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

.PHONY: up
up: ## Start the full stack
	$(COMPOSE) up -d

.PHONY: logs
logs: ## Follow Sablier logs
	$(COMPOSE) logs -f sablier

.PHONY: start
start: ## Request the 'demo' group — triggers a "started" webhook
	curl -s "http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m&timeout=30s" | jq .

.PHONY: receiver-logs
receiver-logs: ## Show the raw webhook payloads received by the echo server
	$(COMPOSE) logs -f webhook-receiver

.PHONY: status
status: ## Show running container status
	$(COMPOSE) ps -a

.PHONY: down
down: ## Tear down the stack
	$(COMPOSE) down
