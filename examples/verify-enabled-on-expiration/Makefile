SABLIER_URL ?= http://localhost:10000
UNLABELED_NAME := sablier-verify-expiration-unlabeled
RESPONSE_FILE := /tmp/sablier-verify-enabled-on-expiration-response.json

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
	@echo "==> Stopping unlabeled container before requesting a session"
	docker compose stop unlabeled
	@echo "==> Requesting unlabeled container by names"
	curl -fsS '$(SABLIER_URL)/api/strategies/blocking?names=$(UNLABELED_NAME)&session_duration=3s&timeout=10s' > "$(RESPONSE_FILE)"
	@echo "==> Waiting for the session to expire"
	sleep 6
	@status=$$(docker inspect -f '{{.State.Status}}' "$(UNLABELED_NAME)"); \
	test "$$status" = "running"; \
	echo "OK: expired unlabeled instance was inspected and left running"
