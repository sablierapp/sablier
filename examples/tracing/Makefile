.PHONY: help start start-simple stop clean logs

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

start: ## Start the full observability stack (Jaeger, Prometheus, Grafana, Collector)
	docker-compose up -d

start-simple: ## Start simple setup (Sablier + Jaeger only)
	docker-compose up -d sablier jaeger whoami

stop: ## Stop all services
	docker-compose down

clean: ## Stop all services and remove volumes
	docker-compose down -v

logs: ## Show logs from all services
	docker-compose logs -f

logs-sablier: ## Show Sablier logs
	docker-compose logs -f sablier

logs-jaeger: ## Show Jaeger logs
	docker-compose logs -f jaeger
