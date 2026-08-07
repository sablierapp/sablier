# Custom Theme with Asset Bundling

This example demonstrates Sablier's **asset bundling** feature. A custom theme
is built from multiple source files — HTML, CSS, JavaScript, an SVG logo, and an
animated GIF — that Sablier reads and inlines at startup, producing a single
self-contained loading page.

## Theme structure

```
themes/
├── my-theme.html                      ← Go template; references the assets below
├── css/
│   └── style.css                      ← inlined as <style>…</style>
├── js/
│   └── app.js                         ← inlined as <script>…</script>
└── imgs/
    ├── sablier-icon-color.svg         ← inlined as <img src="data:image/svg+xml;base64,…">
    └── sablier-rotating-optimized.gif ← inlined as <img src="data:image/gif;base64,…">
```

The images under `themes/imgs/` are **not committed** to the repository; they are
downloaded from the [official Sablier artwork repository](https://github.com/sablierapp/artwork)
the first time you run `make up`.

At startup Sablier walks the themes directory, detects relative asset references
in each `.html` file, reads the referenced files, and replaces the tags with
their inlined equivalents.  The browser receives a single HTML document with no
external dependencies.

Absolute URLs (e.g. the Google Fonts link in this theme) are passed through
unchanged.

## Prerequisites

- Docker with Compose
- `curl` (to download the official artwork on first run)

## Running the example

```bash
# Download artwork, then start Sablier (app container is stopped).
make up

# Open the URL below in a browser to see the custom theme:
#   http://localhost:10000/api/strategies/dynamic?group=myapp&session_duration=1m&theme=my-theme&show_details=true
make demo

# Tear down
make down
```

## What you will see

The browser renders `my-theme.html` with:

- The **Sablier color logo** (SVG) bundled from `imgs/sablier-icon-color.svg`
- The **Sablier rotating animation** (GIF) bundled from `imgs/sablier-rotating-optimized.gif`
- Custom **dark-mode styles** bundled from `css/style.css`
- A **live elapsed-time counter** in the tab title, driven by `js/app.js`
- A table of instance states when `show_details=true`

None of these assets are fetched separately — everything is embedded in the HTML
that Sablier sends.

## Customising the theme

Replace or edit any file under `themes/` and restart Sablier (`make down && make up`).
The bundler re-reads every file at startup, so changes take effect after a restart.

To add your own image formats (JPEG, WebP, PNG, ICO) drop the file into `imgs/`
and reference it with a relative `<img src="imgs/your-file.ext">` tag — the MIME
type is detected automatically from the file extension.
