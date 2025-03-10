PLATFORMS := linux/amd64 linux/arm64 linux/arm/v7 linux/arm

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
VERSION = draft

# Version info for binaries
GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
BUILDTIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILDUSER := $(shell whoami)@$(shell hostname)

VPREFIX := github.com/sablierapp/sablier/pkg/version
GO_LDFLAGS := -s -w -X $(VPREFIX).Branch=$(GIT_BRANCH) -X $(VPREFIX).Version=$(VERSION) -X $(VPREFIX).Revision=$(GIT_REVISION) -X $(VPREFIX).BuildUser=$(BUILDUSER) -X $(VPREFIX).BuildDate=$(BUILDTIME)

$(PLATFORMS):
	CGO_ENABLED=0 GOOS=$(os) GOARCH=$(arch) go build -trimpath -tags=nomsgpack -v -ldflags="${GO_LDFLAGS}" -o 'sablier_$(VERSION)_$(os)-$(arch)' .

run:
	go run ./cmd/sablier start --storage.file=state.json --logging.level=debug

gen:
	go generate -v ./...

build:
	go build -v ./cmd/sablier

test:
	go test -v ./...

plugins: build-plugin-traefik test-plugin-traefik build-plugin-caddy test-plugin-caddy

build-plugin-traefik:
	cd plugins/traefik && go build -v .

test-plugin-traefik:
	cd plugins/traefik && go test -v ./...

build-plugin-caddy:
	cd plugins/caddy && go build -v .

test-plugin-caddy:
	cd plugins/caddy && go test -v .

.PHONY: docker
docker:
	docker build --build-arg BUILDTIME=$(BUILDTIME) --build-arg VERSION=$(VERSION) --build-arg REVISION=$(GIT_REVISION) -t sablierapp/sablier:local .

caddy:
	docker build -t caddy:local plugins/caddy

release: $(PLATFORMS)

proxywasm:
	go generate ./plugins/proxywasm
	env GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o ./plugins/proxywasm/sablierproxywasm.wasm ./plugins/proxywasm
	cp ./plugins/proxywasm/sablierproxywasm.wasm ./sablierproxywasm_$(VERSION).wasm

.PHONY: release $(PLATFORMS)

LAST = 0.0.0
NEXT = 1.0.0
update-doc-version:
	find . -type f \( -name "*.md" -o -name "*.yml" \) -exec sed -i 's/sablierapp\/sablier:$(LAST)/sablierapp\/sablier:$(NEXT)/g' {} +

update-doc-version-middleware:
	find . -type f \( -name "*.md" -o -name "*.yml" \) -exec sed -i 's/version: "v$(LAST)"/version: "v$(NEXT)"/g' {} +
	find . -type f \( -name "*.md" -o -name "*.yml" \) -exec sed -i 's/version=v$(LAST)/version=v$(NEXT)/g' {} +
	sed -i 's/SABLIER_VERSION=v$(LAST)/SABLIER_VERSION=v$(NEXT)/g' plugins/caddy/remote.Dockerfile
	sed -i 's/v$(LAST)/v$(NEXT)/g' plugins/caddy/README.md

.PHONY: docs
docs:
	npx --yes docsify-cli serve docs

# End to end tests
e2e: e2e-caddy e2e-nginx e2e-traefik

## Caddy
e2e-caddy-docker:
	cd plugins/caddy/e2e/docker && bash ./run.sh
	
e2e-caddy-swarm:
	cd plugins/caddy/e2e/docker_swarm && bash ./run.sh

# e2e-caddy-kubernetes:
#   	cd plugins/caddy/e2e/kubernetes && bash ./run.sh

e2e-caddy: e2e-caddy-docker e2e-caddy-swarm # e2e-caddy-kubernetes

## NGinx
e2e-nginx-docker:
	cd plugins/nginx/e2e/docker && bash ./run.sh
	
e2e-nginx-swarm:
	cd plugins/nginx/e2e/docker_swarm && bash ./run.sh

e2e-nginx-kubernetes:
	cd plugins/nginx/e2e/kubernetes && bash ./run.sh

e2e-nginx: e2e-nginx-docker e2e-nginx-swarm e2e-nginx-kubernetes

## Traefik
e2e-traefik-docker:
	cd plugins/traefik/e2e/docker && bash ./run.sh
	
e2e-traefik-swarm:
	cd plugins/traefik/e2e/docker_swarm && bash ./run.sh

e2e-traefik-kubernetes:
	cd plugins/traefik/e2e/kubernetes && bash ./run.sh

e2e-traefik: e2e-traefik-docker e2e-traefik-swarm e2e-traefik-kubernetes
