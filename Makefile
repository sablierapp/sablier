.PHONY: run gen build test lint fmt docker docs

.DEFAULT_GOAL := build

export GOFLAGS=-tags=nomsgpack

run:
	go run ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

schema:
	go run ./cmd/schemagen -out docs/theme.schema.json

check-schema:
	@go run ./cmd/schemagen | diff - docs/theme.schema.json || (echo "docs/theme.schema.json is out of date. Run 'make schema' to regenerate."; exit 1)

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
	npx --yes docsify-cli serve docs