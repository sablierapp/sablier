.DEFAULT_GOAL := help

COMPOSE := docker compose

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

.PHONY: up
up: ## Start the full stack (managed gets a seeded session instead of being stopped)
	$(COMPOSE) up -d

.PHONY: logs
logs: ## Follow Sablier logs
	$(COMPOSE) logs -f sablier

.PHONY: status
status: ## Show running container status
	$(COMPOSE) ps -a

.PHONY: start-managed
start-managed: ## Start 'managed' externally to demonstrate the continuous watch
	$(COMPOSE) start managed
	@echo ""
	@echo "Watch the Sablier logs — 'managed' receives a new 1m session and"
	@echo "will be stopped about a minute later if it receives no traffic."
	@echo "Run: make logs"

.PHONY: down
down: ## Tear down the stack
	$(COMPOSE) down
