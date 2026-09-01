.PHONY: install uninstall start status logs

XDG_UNITS := $(HOME)/.config/systemd/user
SABLIER_UNIT := $(XDG_UNITS)/sablier.service
DEMO_UNIT := $(XDG_UNITS)/demo.service

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

# Remove the demo unit and Sablier configuration.
uninstall:
	-systemctl --user stop sablier.service
	-systemctl --user disable sablier.service
	-systemctl --user disable demo.service
	-rm -f $(DEMO_UNIT) $(SABLIER_UNIT) $(HOME)/.config/sablier/sablier.yaml
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

