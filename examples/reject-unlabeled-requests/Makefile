SABLIER_URL ?= http://localhost:10000
UNMANAGED_NAME := sablier-reject-unlabeled-unmanaged
RESPONSE_FILE := /tmp/sablier-reject-unlabeled-response.json

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
	@echo "==> Requesting unmanaged container by names; expect rejection"
	@code=$$(curl -s -o "$(RESPONSE_FILE)" -w "%{http_code}" '$(SABLIER_URL)/api/strategies/blocking?names=$(UNMANAGED_NAME)&session_duration=5s&timeout=5s'); \
	cat "$(RESPONSE_FILE)"; echo; \
	test "$$code" = "500"; \
	grep -q "instance $(UNMANAGED_NAME) is not managed by sablier" "$(RESPONSE_FILE)"; \
	echo "OK: request was rejected"
