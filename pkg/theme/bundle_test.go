package theme

import (
	"encoding/base64"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleHTML_InlinesCSS(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<html><head><link rel="stylesheet" href="style.css"></head><body></body></html>`)},
		"style.css":  {Data: []byte(`body { color: red; }`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "<style>body { color: red; }</style>")
	assert.NotContains(t, result, "<link")
}

func TestBundleHTML_InlinesCSSFromSubdir(t *testing.T) {
	f := fstest.MapFS{
		"themes/theme.html":    {Data: []byte(`<link rel="stylesheet" href="css/style.css">`)},
		"themes/css/style.css": {Data: []byte(`.x { color: blue; }`)},
	}
	result, err := bundleHTML(f, "themes/theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "<style>.x { color: blue; }</style>")
}

func TestBundleHTML_InlinesScript(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<html><body><script src="app.js"></script></body></html>`)},
		"app.js":     {Data: []byte(`console.log("hello");`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `<script>console.log("hello");</script>`)
	assert.NotContains(t, result, `src=`)
}

func TestBundleHTML_LeavesInlineScriptUntouched(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<script>var x = 1;</script>`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `<script>var x = 1;</script>`)
}

func TestBundleHTML_InlinesImage(t *testing.T) {
	imgData := []byte{0x89, 0x50, 0x4e, 0x47} // PNG magic bytes
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<img src="logo.png" alt="logo">`)},
		"logo.png":   {Data: imgData},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	expected := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgData)
	assert.Contains(t, result, expected)
	assert.NotContains(t, result, `src="logo.png"`)
}

func TestBundleHTML_InlinesSVGImage(t *testing.T) {
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`)
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<img src="icon.svg" alt="icon">`)},
		"icon.svg":   {Data: svgData},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "data:image/svg+xml;base64,")
}

func TestBundleHTML_LeavesAbsoluteURLsUntouched(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(
			`<link rel="stylesheet" href="https://cdn.example.com/style.css">` +
				`<img src="https://example.com/logo.png">` +
				`<script src="https://cdn.example.com/app.js"></script>`,
		)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="https://cdn.example.com/style.css"`)
	assert.Contains(t, result, `src="https://example.com/logo.png"`)
	assert.Contains(t, result, `src="https://cdn.example.com/app.js"`)
}

func TestBundleHTML_LeavesProtocolRelativeURLsUntouched(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link rel="stylesheet" href="//cdn.example.com/style.css">`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="//cdn.example.com/style.css"`)
}

func TestBundleHTML_LeavesRootRelativePathsUntouched(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link rel="stylesheet" href="/static/style.css">`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="/static/style.css"`)
}

func TestBundleHTML_LeavesDataURIsUntouched(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<img src="data:image/png;base64,abc123">`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `src="data:image/png;base64,abc123"`)
}

func TestBundleHTML_PreservesTagOnMissingFile(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link rel="stylesheet" href="missing.css">`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="missing.css"`)
}

func TestBundleHTML_CaseInsensitiveAttributes(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<LINK REL="stylesheet" HREF="style.css">`)},
		"style.css":  {Data: []byte(`body {}`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "<style>body {}</style>")
}

func TestBundleHTML_RelAttributeBeforeHref(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link href="style.css" rel="stylesheet">`)},
		"style.css":  {Data: []byte(`h1 {}`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "<style>h1 {}</style>")
}

func TestBundleHTML_MultipleCSSFiles(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(
			"<link rel=\"stylesheet\" href=\"a.css\">\n<link rel=\"stylesheet\" href=\"b.css\">\n",
		)},
		"a.css": {Data: []byte(`.a {}`)},
		"b.css": {Data: []byte(`.b {}`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, "<style>.a {}</style>")
	assert.Contains(t, result, "<style>.b {}</style>")
	assert.Equal(t, 0, strings.Count(result, "<link"))
}

func TestBundleHTML_SkipsNonStylesheetLinks(t *testing.T) {
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link rel="icon" href="favicon.ico">`)},
	}
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="favicon.ico"`)
}

func TestBundleHTML_ErrorOnMissingHTMLFile(t *testing.T) {
	f := fstest.MapFS{}
	_, err := bundleHTML(f, "nonexistent.html")
	assert.Error(t, err)
}

func TestBundleHTML_LeavesTagIntactWhenAssetTooLarge(t *testing.T) {
	// Build a CSS file whose size exceeds the 10 MiB limit.
	huge := make([]byte, maxAssetBytes+1)
	for i := range huge {
		huge[i] = 'x'
	}
	f := fstest.MapFS{
		"theme.html": {Data: []byte(`<link rel="stylesheet" href="huge.css">`)},
		"huge.css":   {Data: huge},
	}
	// The bundler should NOT crash or OOM; it must leave the original tag intact.
	result, err := bundleHTML(f, "theme.html")
	require.NoError(t, err)
	assert.Contains(t, result, `href="huge.css"`, "tag should be preserved when asset exceeds size limit")
	assert.NotContains(t, result, "<style>")
}

func TestIsRelativeRef(t *testing.T) {
	cases := []struct {
		ref      string
		relative bool
	}{
		{"style.css", true},
		{"css/style.css", true},
		{"../style.css", true},
		{"", false},
		{"https://example.com/style.css", false},
		{"http://example.com/style.css", false},
		{"//example.com/style.css", false},
		{"/static/style.css", false},
		{"data:image/png;base64,abc", false},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			assert.Equal(t, tc.relative, isRelativeRef(tc.ref))
		})
	}
}
