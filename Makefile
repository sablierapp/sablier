PLATFORMS := linux/amd64 linux/arm64 linux/arm/v7

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))
version = draft

release: $(PLATFORMS)

$(PLATFORMS):
	GOOS=$(os) GOARCH=$(arch) go build -o 'sablier_$(version)_$(os)-$(arch)' .

.PHONY: release $(PLATFORMS)