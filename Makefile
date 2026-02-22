.PHONY: run gen build test lint fmt docker docs

GO_FLAGS := nomsgpack,remote,exclude_graphdriver_btrfs,containers_image_openpgp

run:
	go run -tags="$(GO_FLAGS)" ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

build:
	go build -tags="$(GO_FLAGS)" -v ./cmd/sablier

test:
	go test -tags="$(GO_FLAGS)" ./...

lint:
	golangci-lint run --build-tags="$(GO_FLAGS)"

fix:
	golangci-lint run --build-tags="$(GO_FLAGS)" --fix

fmt:
	golangci-lint fmt ./...

BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION = draft
GIT_REVISION := $(shell git rev-parse --short HEAD)
docker:
	docker build --build-arg BUILDTIME=$(BUILDTIME) --build-arg VERSION=$(VERSION) --build-arg REVISION=$(GIT_REVISION) -t sablierapp/sablier:local .

docs:
	npx --yes docsify-cli serve docs