// Package docsver centralises how an introduction version ("since") is written
// in the source and rendered in the generated documentation.
//
// A released option carries a concrete tag in its doc comment (for example
// "since: v1.13.0"). A not-yet-released option carries the NextRelease template
// token instead, which renders as a plain "Next release" badge.
//
// Before cutting a release, replace the token with the upcoming tag by hand
// (find-and-replace NEXT_RELEASE -> vX.Y.Z across pkg/config and
// pkg/sablier/labels.go, then run `make cli-docs labels-docs`). Once replaced,
// the concrete tag never changes, so each "since" stays frozen at the version
// the option first shipped in.
package docsver

import (
	"fmt"
	"strings"
)

// NextRelease is the template token used in doc comments for options that have
// not been released yet. Replace it with the upcoming tag (vX.Y.Z) by hand
// before cutting a release; see the package doc for the exact steps.
const NextRelease = "NEXT_RELEASE"

const releaseBaseURL = "https://github.com/sablierapp/sablier/releases/tag/"

// Display returns the human-facing label and an optional release URL for a
// "since" value:
//   - ""            -> ("", "")            no version information
//   - NextRelease   -> ("Next release", "") upcoming, not yet tagged
//   - "vX.Y.Z"      -> ("vX.Y.Z", <release URL>)
//   - anything else -> (value, "")          rendered verbatim, unlinked
func Display(v string) (label, link string) {
	switch {
	case v == "":
		return "", ""
	case v == NextRelease:
		return "Next release", ""
	case strings.HasPrefix(v, "v"):
		return v, releaseBaseURL + v
	default:
		return v, ""
	}
}

// SinceBadge renders the Hextra "since" badge shortcode for a version value: a
// tagged version links to its release ("Since vX.Y.Z"), the NextRelease token
// renders as a plain "Next release" badge, and an unknown value returns "".
func SinceBadge(v string) string {
	label, link := Display(v)
	if label == "" {
		return ""
	}
	content := "Since " + label
	if v == NextRelease {
		content = label // already reads "Next release"
	}
	if link != "" {
		return fmt.Sprintf("{{< badge content=%q link=%q >}}", content, link)
	}
	return fmt.Sprintf("{{< badge content=%q >}}", content)
}
