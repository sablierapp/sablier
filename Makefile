.PHONY: run gen generate check-generate build test lint fmt docker docs schema check-schema cli-docs check-cli-docs labels-docs check-labels-docs openapi check-openapi client check-client test-client metrics-docs check-metrics-docs

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
	go run ./cmd/docgen -out docs/content/reference/cli.md

check-cli-docs:
	@go run ./cmd/docgen | diff - docs/content/reference/cli.md || (echo "docs/content/reference/cli.md is out of date. Run 'make cli-docs' to regenerate."; exit 1)

labels-docs:
	go run ./cmd/labelsgen -src pkg/sablier/labels.go -out docs/content/reference/labels.md

check-labels-docs:
	@go run ./cmd/labelsgen -src pkg/sablier/labels.go | diff - docs/content/reference/labels.md || (echo "docs/content/reference/labels.md is out of date. Run 'make labels-docs' to regenerate."; exit 1)

openapi:
	go run ./cmd/openapigen -out docs/static/openapi.json

check-openapi:
	@go run ./cmd/openapigen | diff - docs/static/openapi.json || (echo "docs/static/openapi.json is out of date. Run 'make openapi' to regenerate."; exit 1)

# The Go client's types in client/ are generated from the OpenAPI spec, so
# regenerate them after `make openapi`.
client:
	cd client && go generate ./...

# Also fails if the client gains a dependency: it must stay standard library only.
check-client:
	@go tool -modfile=tools.mod oapi-codegen -config client/oapi-codegen.yaml docs/static/openapi.json | diff - client/types.gen.go || (echo "client/types.gen.go is out of date. Run 'make client' to regenerate."; exit 1)
	@! grep -q '^require' client/go.mod || (echo "client/go.mod must not require anything: the client is standard library only."; exit 1)

metrics-docs:
	go run ./cmd/metricsgen -out docs/content/features/metrics.md

check-metrics-docs:
	@go run ./cmd/metricsgen | diff - docs/content/features/metrics.md || (echo "docs/content/features/metrics.md is out of date. Run 'make metrics-docs' to regenerate."; exit 1)

# Regenerate every committed generated file: the CLI, labels and metrics
# reference docs, the theme JSON schema, the OpenAPI spec and its Go client.
generate: schema cli-docs labels-docs openapi client metrics-docs

# Verify every committed generated file is up to date. Used in CI; run
# `make generate` and commit the result if it fails.
check-generate: check-schema check-cli-docs check-labels-docs check-openapi check-client check-metrics-docs

.PHONY: build
build:
	goreleaser build --single-target --snapshot --clean --output .

test: test-client
	go test ./...

# client/ is a separate module, so `go test ./...` from the root skips it.
test-client:
	cd client && go test ./...

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