package theme

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"path"
	"regexp"
	"strings"
)

// maxAssetBytes is the upper bound on how many bytes a single bundled asset
// (CSS, JS, or image) may contain. Files larger than this limit are skipped
// and their original HTML tags are preserved, preventing runaway memory
// allocation from accidental (or deliberate) inclusion of huge binary files.
const maxAssetBytes = 10 << 20 // 10 MiB

// safeReadFile reads at most maxAssetBytes from name inside f.
// It returns an error when the file exceeds the limit so the caller can
// leave the original tag unchanged rather than crashing or OOM-ing.
//
// Security note: path traversal via ".." is already blocked by fs.ValidPath,
// which the fs.FS contract requires every Open implementation to enforce.
// Symlink following must be prevented at the fs.FS construction site (see
// noSymlinkFS) because fs.ReadFile itself has no way to detect symlinks.
func safeReadFile(f fs.FS, name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	// Read one byte beyond the limit so we can detect over-size files.
	data, err := io.ReadAll(io.LimitReader(file, maxAssetBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAssetBytes {
		return nil, fmt.Errorf("asset %q exceeds the %d-byte bundling limit", name, maxAssetBytes)
	}
	return data, nil
}

var (
	// linkTagRe matches a <link ...> tag (self-closing or not).
	linkTagRe = regexp.MustCompile(`(?i)<link\b[^>]*>`)
	// hrefAttrRe extracts the href attribute value from a tag string.
	hrefAttrRe = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)
	// relStylesheetRe matches a rel="stylesheet" (or rel='stylesheet') attribute.
	relStylesheetRe = regexp.MustCompile(`(?i)\brel\s*=\s*["']stylesheet["']`)

	// scriptTagRe matches a <script ...>...</script> block, including multiline bodies.
	scriptTagRe = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	// srcAttrRe extracts the src attribute value from a tag string.
	srcAttrRe = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)

	// imgTagRe matches an <img ...> tag.
	imgTagRe = regexp.MustCompile(`(?i)<img\b[^>]*>`)
)

// isRelativeRef reports whether ref is a relative asset reference that should
// be inlined. Absolute URLs (http://, //), root-relative paths (/), and data
// URIs are all left untouched.
func isRelativeRef(ref string) bool {
	return ref != "" &&
		!strings.HasPrefix(ref, "data:") &&
		!strings.HasPrefix(ref, "/") &&
		!strings.HasPrefix(ref, "//") &&
		!strings.Contains(ref, "://")
}

// bundleHTML reads the HTML file at htmlPath from f and returns a version where
// every relative asset reference is replaced with its inlined equivalent:
//
//   - <link rel="stylesheet" href="path.css"> → <style>/* css content */</style>
//   - <script src="path.js"></script>         → <script>/* js content */</script>
//   - <img src="path.png">                    → <img src="data:image/png;base64,...">
//
// Absolute URLs, root-relative paths, and data URIs are left untouched.
// If a referenced file cannot be read from the FS the original tag is preserved,
// so a missing asset degrades gracefully rather than preventing the theme from loading.
func bundleHTML(f fs.FS, htmlPath string) (string, error) {
	raw, err := fs.ReadFile(f, htmlPath)
	if err != nil {
		return "", err
	}
	content := string(raw)

	// Compute the directory that contains the HTML file so that relative asset
	// references are resolved against it.
	dir := path.Dir(htmlPath)
	if dir == "." {
		dir = ""
	}

	assetPath := func(ref string) string {
		if dir == "" {
			return ref
		}
		return dir + "/" + ref
	}

	// --- Inline <link rel="stylesheet" href="..."> tags. ---
	content = linkTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		if !relStylesheetRe.MatchString(tag) {
			return tag
		}
		m := hrefAttrRe.FindStringSubmatch(tag)
		if m == nil || !isRelativeRef(m[1]) {
			return tag
		}
		css, err := safeReadFile(f, assetPath(m[1]))
		if err != nil {
			return tag // leave as-is; the browser will get a broken link
		}
		return "<style>" + string(css) + "</style>"
	})

	// --- Inline <script src="..."></script> tags that have no inline body. ---
	content = scriptTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		parts := scriptTagRe.FindStringSubmatch(tag)
		if parts == nil {
			return tag
		}
		attrs, body := parts[1], parts[2]
		if strings.TrimSpace(body) != "" {
			// Script already has inline content; leave it alone.
			return tag
		}
		m := srcAttrRe.FindStringSubmatch(attrs)
		if m == nil || !isRelativeRef(m[1]) {
			return tag
		}
		js, err := safeReadFile(f, assetPath(m[1]))
		if err != nil {
			return tag
		}
		return "<script>" + string(js) + "</script>"
	})

	// --- Inline <img src="..."> tags as base64 data URIs. ---
	content = imgTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		m := srcAttrRe.FindStringSubmatch(tag)
		if m == nil || !isRelativeRef(m[1]) {
			return tag
		}
		imgData, err := safeReadFile(f, assetPath(m[1]))
		if err != nil {
			return tag
		}
		mimeType := mime.TypeByExtension(path.Ext(m[1]))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		dataURI := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(imgData)
		// Replace only the src attribute value; preserve all other attributes.
		return strings.Replace(tag, m[0], `src="`+dataURI+`"`, 1)
	})

	return content, nil
}
