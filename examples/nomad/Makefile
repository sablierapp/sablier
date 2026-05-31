.PHONY: help up down logs test clean health status nomad-ui

# Sablier image (can be overridden)
SABLIER_IMAGE ?= sablierapp/sablier:1.10.1
export SABLIER_IMAGE

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Environment variables:'
	@echo '  SABLIER_IMAGE    Docker image to use (default: $(SABLIER_IMAGE))'

up: ## Start Nomad cluster and deploy jobs
	docker compose up -d
	@echo "Waiting for Nomad to be ready..."
	@timeout=30; \
	while [ $$timeout -gt 0 ]; do \
		if docker exec nomad nomad status > /dev/null 2>&1; then \
			echo "✓ Nomad is ready"; \
			break; \
		fi; \
		timeout=$$((timeout - 1)); \
		sleep 1; \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "✗ Nomad failed to start"; \
		exit 1; \
	fi
	@echo "Deploying jobs..."
	@docker exec nomad nomad job run /nomad/jobs/sablier.nomad.hcl
	@docker exec nomad nomad job run /nomad/jobs/nginx.nomad.hcl
	@docker exec nomad nomad job run /nomad/jobs/whoami.nomad.hcl
	@sleep 5
	@echo "✓ All jobs deployed"

down: ## Stop and remove everything
	docker compose down -v

logs: ## Show logs from all services
	docker compose logs -f

logs-%: ## Show logs from specific service (e.g., make logs-sablier)
	docker compose logs -f $*

test: up ## Run tests
	@echo "=== Testing Nomad Provider Example ==="
	@echo ""
	@echo "→ Waiting for Sablier to be healthy..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if curl -f -s http://localhost:10000/health > /dev/null 2>&1; then \
			echo "✓ Sablier is healthy"; \
			break; \
		fi; \
		timeout=$$((timeout - 1)); \
		sleep 1; \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "✗ Sablier health check failed"; \
		docker compose logs sablier; \
		exit 1; \
	fi
	@echo ""
	@echo "→ Checking whoami is scaled to 0..."
	@sleep 5
	@count=$$(docker exec nomad nomad job status whoami 2>/dev/null | grep -A 2 "Task Group" | grep "whoami" | awk '{print $$4}' || echo "0"); \
	if [ "$$count" != "0" ]; then \
		echo "✗ Whoami should be at count 0, got $$count"; \
		exit 1; \
	fi
	@echo "✓ Whoami is at count 0"
	@echo ""
	@echo "→ Making request (dynamic strategy)..."
	@http_code=$$(curl -s -w "%{http_code}" -o /dev/null http://localhost:8080/whoami); \
	if [ "$$http_code" -ne 200 ]; then \
		echo "✗ Expected HTTP 200, got $$http_code"; \
		exit 1; \
	fi
	@echo "✓ Received HTTP 200"
	@echo ""
	@echo "→ Waiting for whoami to scale up..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		count=$$(docker exec nomad nomad job status whoami 2>/dev/null | grep "running" | wc -l || echo "0"); \
		if [ "$$count" -gt 0 ]; then \
			echo "✓ Whoami scaled up"; \
			break; \
		fi; \
		timeout=$$((timeout - 1)); \
		sleep 1; \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "✗ Whoami failed to scale up"; \
		docker exec nomad nomad job status whoami; \
		exit 1; \
	fi
	@echo ""
	@sleep 5
	@echo "→ Testing blocking strategy..."
	@response=$$(curl -s http://localhost:8080/blocking); \
	if echo "$$response" | grep -q "whoami"; then \
		echo "✓ Blocking strategy works"; \
	else \
		echo "✗ Blocking strategy failed"; \
		exit 1; \
	fi
	@echo ""
	@echo "✓ All tests passed!"

clean: down ## Clean up everything

health: ## Check Sablier health
	@curl -f http://localhost:10000/health && echo "" || (echo "Sablier is not healthy" && exit 1)

status: ## Show job status
	@docker exec nomad nomad status
	@echo ""
	@echo "Whoami job:"
	@docker exec nomad nomad job status whoami

try: up ## Quick manual test
	@echo "Testing dynamic strategy..."
	@curl http://localhost:8080/whoami
	@echo ""
	@echo ""
	@echo "Job status:"
	@sleep 2
	@docker exec nomad nomad job status whoami

nomad-ui: ## Open Nomad UI in browser
	@echo "Opening Nomad UI at http://localhost:4646"
	@which xdg-open > /dev/null && xdg-open http://localhost:4646 || \
	which open > /dev/null && open http://localhost:4646 || \
	echo "Please open http://localhost:4646 in your browser"
