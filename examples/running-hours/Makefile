.DEFAULT_GOAL := help

COMPOSE := docker compose

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

.PHONY: up
up: ## Start the stack and stop office-app for a cold start demo
	$(COMPOSE) up -d
	$(COMPOSE) stop office-app

.PHONY: request
request: ## Trigger a blocking request for office-app (starts if needed)
	curl -sf "http://localhost:10000/api/strategies/blocking?group=office-app&session_duration=1m&timeout=1m" | jq .

.PHONY: status
status: ## Show service status
	$(COMPOSE) ps -a

.PHONY: logs
logs: ## Follow Sablier logs
	$(COMPOSE) logs -f sablier

.PHONY: down
down: ## Tear down the stack
	$(COMPOSE) down
