SABLIER_URL ?= http://localhost:10000
WHOAMI_NAME := sablier-group-discovery-whoami

.PHONY: up down logs wait-sablier test

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f sablier

wait-sablier:
	@for i in $$(seq 1 30); do \
		if curl -fsS "$(SABLIER_URL)/health" >/dev/null; then exit 0; fi; \
		sleep 1; \
	done; \
	echo "sablier did not become healthy"; \
	docker compose logs sablier; \
	exit 1

test: up wait-sablier
	@echo "==> Waiting for the discovery-created session to expire"
	sleep 12
	@status=$$(docker inspect -f '{{.State.Status}}' "$(WHOAMI_NAME)"); \
	test "$$status" != "running"; \
	echo "OK: discovered group session expired and stopped $(WHOAMI_NAME)"
