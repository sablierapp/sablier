.PHONY: run gen build test lint fmt docker docs schema check-schema cli-docs check-cli-docs labels-docs check-labels-docs openapi check-openapi metrics-docs check-metrics-docs

.DEFAULT_GOAL := build

export GOFLAGS=-tags=nomsgpack

run:
	go run ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

schema:
	go run ./cmd/schemagen -out docs/static/theme.schema.json

check-schema:
	@go run ./cmd/schemagen | diff - docs/static/theme.schema.json || (echo "docs/static/theme.schema.json is out of date. Run 'make schema' to regenerate."; exit 1)

cli-docs:
	go run ./cmd/docgen -out docs/content/cli.md

check-cli-docs:
	@go run ./cmd/docgen | diff - docs/content/cli.md || (echo "docs/content/cli.md is out of date. Run 'make cli-docs' to regenerate."; exit 1)

labels-docs:
	go run ./cmd/labelsgen -src pkg/sablier/labels.go -out docs/content/labels.md

check-labels-docs:
	@go run ./cmd/labelsgen -src pkg/sablier/labels.go | diff - docs/content/labels.md || (echo "docs/content/labels.md is out of date. Run 'make labels-docs' to regenerate."; exit 1)

openapi:
	go run ./cmd/openapigen -out docs/static/openapi.json

check-openapi:
	@go run ./cmd/openapigen | diff - docs/static/openapi.json || (echo "docs/static/openapi.json is out of date. Run 'make openapi' to regenerate."; exit 1)

metrics-docs:
	go run ./cmd/metricsgen -out docs/content/metrics.md

check-metrics-docs:
	@go run ./cmd/metricsgen | diff - docs/content/metrics.md || (echo "docs/content/metrics.md is out of date. Run 'make metrics-docs' to regenerate."; exit 1)

.PHONY: build
build:
	goreleaser build --single-target --snapshot --clean --output .

test:
	go test ./...

lint:
	golangci-lint run

fix:
	golangci-lint run --fix

fmt:
	golangci-lint fmt ./...

docker:
	goreleaser release --snapshot --clean --skip=publish

docs:
	cd docs && hugo server