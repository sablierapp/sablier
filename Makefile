PLATFORMS := linux/amd64 linux/arm64 linux/arm/v7

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
version = draft

# Version info for binaries
GIT_REVISION := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

VPREFIX := github.com/acouvreur/sablier/version
GO_LDFLAGS := -X $(VPREFIX).Branch=$(GIT_BRANCH) -X $(VPREFIX).Version=$(version) -X $(VPREFIX).Revision=$(GIT_REVISION) -X $(VPREFIX).BuildUser=$(shell whoami)@$(shell hostname) -X $(VPREFIX).BuildDate=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

$(PLATFORMS):
	GIN_MODE=release GOOS=$(os) GOARCH=$(arch) go build -ldflags="${GO_LDFLAGS}" -o 'sablier_$(version)_$(os)-$(arch)' .


release: $(PLATFORMS)
.PHONY: release $(PLATFORMS)