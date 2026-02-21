.PHONY: run gen build test lint fmt docker docs

run:
	go run ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

build:
	go build -tags="nomsgpack,remote,exclude_graphdriver_btrfs,containers_image_openpgp" -v ./cmd/sablier

test:
	go test -tags="nomsgpack,remote,exclude_graphdriver_btrfs,containers_image_openpgp" ./...

lint:
	golangci-lint run --build-tags="nomsgpack,remote,exclude_graphdriver_btrfs,containers_image_openpgp" ./...

fmt:
	golangci-lint fmt ./...

docker:
	docker build --build-arg BUILDTIME=$(BUILDTIME) --build-arg VERSION=$(VERSION) --build-arg REVISION=$(GIT_REVISION) -t sablierapp/sablier:local .

docs:
	npx --yes docsify-cli serve docs