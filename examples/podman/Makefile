.PHONY: help up down logs test clean health status socket-enable

# Detect if rootless or rootful
PODMAN_CMD := podman-compose
SOCKET_PATH := $(shell if [ "$$EUID" -eq 0 ]; then echo "/run/podman/podman.sock"; else echo "$$XDG_RUNTIME_DIR/podman/podman.sock"; fi)
# Sablier image (can be overridden)
SABLIER_IMAGE ?= docker.io/sablierapp/sablier:1.10.1
export SABLIER_IMAGE

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ''
	@echo 'Environment variables:'
	@echo '  SABLIER_IMAGE    Docker image to use (default: $(SABLIER_IMAGE))'

socket-enable: ## Enable Podman socket
	@if [ "$$EUID" -eq 0 ]; then \
		echo "Enabling rootful podman socket..."; \
		systemctl enable --now podman.socket; \
	else \
		echo "Enabling rootless podman socket..."; \
		systemctl --user enable --now podman.socket; \
	fi
	@echo "Socket enabled at: $(SOCKET_PATH)"

up: ## Start the example
	$(PODMAN_CMD) up -d
	@echo "Waiting for services to be ready..."
	@sleep 5

down: ## Stop and remove containers
	$(PODMAN_CMD) down -v

logs: ## Show logs from all services
	$(PODMAN_CMD) logs -f

logs-%: ## Show logs from specific service
	$(PODMAN_CMD) logs -f $*

test: up ## Run tests
	@echo "=== Testing Podman Provider Example ==="
	@echo ""
	@echo "→ Checking socket..."
	@if [ ! -S "$(SOCKET_PATH)" ]; then \
		echo "✗ Podman socket not found at $(SOCKET_PATH)"; \
		echo "Run: make socket-enable"; \
		exit 1; \
	fi
	@echo "✓ Socket found at $(SOCKET_PATH)"
	@echo ""
	@echo "→ Waiting for Sablier to be healthy..."
	@timeout=30; \
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
		$(PODMAN_CMD) logs sablier; \
		exit 1; \
	fi
	@echo ""
	@echo "→ Checking whoami container is stopped..."
	@sleep 2
	@if podman ps --format '{{.Names}}' | grep -q "^whoami$$"; then \
		echo "✗ Whoami should be stopped initially"; \
		exit 1; \
	fi
	@echo "✓ Whoami is stopped"
	@echo ""
	@echo "→ Making request (dynamic strategy)..."
	@http_code=$$(curl -s -w "%{http_code}" -o /dev/null http://localhost:8080/whoami); \
	if [ "$$http_code" -ne 200 ]; then \
		echo "✗ Expected HTTP 200, got $$http_code"; \
		exit 1; \
	fi
	@echo "✓ Received HTTP 200"
	@echo ""
	@echo "→ Waiting for whoami to start..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if podman ps --format '{{.Names}}' | grep -q "^whoami$$"; then \
			echo "✓ Whoami started"; \
			break; \
		fi; \
		timeout=$$((timeout - 1)); \
		sleep 1; \
	done; \
	if [ $$timeout -eq 0 ]; then \
		echo "✗ Whoami failed to start"; \
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

status: ## Show container status
	@$(PODMAN_CMD) ps
	@echo ""
	@echo "Whoami container:"
	@podman ps -a --filter name=whoami --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

try: up ## Quick manual test
	@echo "Testing dynamic strategy..."
	@curl http://localhost:8080/whoami
	@echo ""
	@echo ""
	@echo "Container status:"
	@sleep 2
	@podman ps --filter name=whoami
