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

VPREFIX := github.com/acouvreur/sablier/version
GO_LDFLAGS := -X $(VPREFIX).Branch=$(GIT_BRANCH) -X $(VPREFIX).Version=$(VERSION) -X $(VPREFIX).Revision=$(GIT_REVISION) -X $(VPREFIX).BuildUser=$(BUILDUSER) -X $(VPREFIX).BuildDate=$(BUILDTIME)

$(PLATFORMS):
	CGO_ENABLED=0 GOOS=$(os) GOARCH=$(arch) go build -tags=nomsgpack -v -ldflags="${GO_LDFLAGS}" -o 'sablier_$(VERSION)_$(os)-$(arch)' .

release: $(PLATFORMS)
.PHONY: release $(PLATFORMS)