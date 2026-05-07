.PHONY: run gen build test lint fmt docker docs

.DEFAULT_GOAL := build

export GOFLAGS=-tags=nomsgpack,remote,exclude_graphdriver_btrfs,containers_image_openpgp

run:
	go run ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

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