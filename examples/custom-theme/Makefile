.PHONY: up down logs demo

ARTWORK_BASE := https://raw.githubusercontent.com/sablierapp/artwork/refs/heads/main

# Download official artwork then start Sablier (app container is stopped).
up: themes/imgs/sablier-icon-color.svg themes/imgs/sablier-rotating-optimized.gif
	docker compose up -d
	docker compose stop app

down:
	docker compose down

logs:
	docker compose logs -f sablier

# Open this URL in a browser to see the custom theme with bundled assets.
demo:
	@echo "==> Open the URL below in a browser:"
	@echo "    http://localhost:10000/api/strategies/dynamic?group=myapp&session_duration=1m&theme=my-theme&show_details=true"
	@echo ""
	@echo "==> Or trigger it directly (returns rendered HTML):"
	curl -s "http://localhost:10000/api/strategies/dynamic?group=myapp&session_duration=1m&theme=my-theme" | head -5

# ---------------------------------------------------------------------------
# Artwork — downloaded once from the official Sablier artwork repository.
# These files are bundled into the theme HTML by Sablier at startup.
# ---------------------------------------------------------------------------

themes/imgs/sablier-icon-color.svg:
	@mkdir -p themes/imgs
	curl -sSfL '$(ARTWORK_BASE)/icon/sablier-icon-color.svg' -o $@

themes/imgs/sablier-rotating-optimized.gif:
	@mkdir -p themes/imgs
	curl -sSfL '$(ARTWORK_BASE)/animated/sablier-rotating-optimized.gif' -o $@
