.PHONY: download install add uninstall start status logs

SABLIER_VERSION := 1.19.1 # x-release-please-version
VERSION := $(strip $(SABLIER_VERSION))
ARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/' -e 's/armv7l/armv7/')

XDG_UNITS := $(HOME)/.config/systemd/user
SABLIER_UNIT := $(XDG_UNITS)/sablier.service
DEMO_UNIT := $(XDG_UNITS)/demo.service
# Every example unit in this folder, except Sablier itself.
EXAMPLE_UNITS := $(filter-out sablier.service,$(wildcard *.service))

# Download the Sablier release binary to ~/.local/bin/sablier.
download:
	install -d $(HOME)/.local/bin
	curl -fsSL https://github.com/sablierapp/sablier/releases/download/v$(VERSION)/sablier-$(VERSION)-linux-$(ARCH).tar.gz \
		| tar -xz -C $(HOME)/.local/bin sablier

# Install the demo unit and Sablier, then start Sablier.
# Uses the systemd *user* instance, so no root / sudo is required.
install:
	install -d $(XDG_UNITS) $(HOME)/.config/sablier
	install -m 0644 sablier.yaml $(HOME)/.config/sablier/sablier.yaml
	install -m 0644 sablier.service $(SABLIER_UNIT)
	install -m 0644 demo.service $(DEMO_UNIT)
	systemctl --user daemon-reload
	systemctl --user enable sablier.service
	systemctl --user enable demo.service
	systemctl --user start sablier.service

# Install and enable one more example unit: make add UNIT=ready-after.service
add:
	@test -n "$(UNIT)" || { echo "usage: make add UNIT=<name>.service"; exit 1; }
	install -m 0644 $(UNIT) $(XDG_UNITS)/$(UNIT)
	systemctl --user daemon-reload
	systemctl --user enable $(UNIT)

# Remove Sablier, its configuration and every example unit.
uninstall:
	-systemctl --user stop sablier.service
	-systemctl --user disable sablier.service
	-for unit in $(EXAMPLE_UNITS); do \
		if [ -f $(XDG_UNITS)/$$unit ]; then systemctl --user disable --now $$unit; rm -f $(XDG_UNITS)/$$unit; fi; \
	done
	-rm -f $(SABLIER_UNIT) $(HOME)/.config/sablier/sablier.yaml
	systemctl --user daemon-reload

# Ask Sablier to start the demo group (blocking strategy, 1 minute session).
start:
	@echo ">>> Requesting demo group (blocking, 1m session)..."
	time curl -sf "http://localhost:10000/api/strategies/blocking?group=demo&session_duration=1m" | jq .

# Show whether the demo unit is currently running.
status:
	@echo -n "demo.service: "
	-systemctl --user status demo.service

# Follow Sablier logs.
logs:
	journalctl --user -u sablier -f
