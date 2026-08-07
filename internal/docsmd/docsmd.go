// Package docsmd holds small Markdown helpers shared by the reference-doc
// generators (cmd/docgen and cmd/labelsgen) so the two stay identical instead
// of carrying copy-pasted formatting logic.
package docsmd

import "strings"

// abbreviations are tokens whose trailing period does not end a sentence, so the
// index truncation must not stop on them.
var abbreviations = []string{"e.g", "i.e", "etc", "vs"}

// FirstSentence returns the first sentence of s (up to a sentence-ending ". "),
// keeping index tables compact. Common abbreviations (e.g., i.e.) do not end a
// sentence.
func FirstSentence(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	for i := 0; i+1 < len(s); i++ {
		if s[i] != '.' || s[i+1] != ' ' {
			continue
		}
		fields := strings.Fields(s[:i])
		last := ""
		if len(fields) > 0 {
			last = strings.TrimLeft(fields[len(fields)-1], "([\"'")
		}
		abbrev := false
		for _, a := range abbreviations {
			if strings.EqualFold(last, a) {
				abbrev = true
				break
			}
		}
		if !abbrev {
			return s[:i+1]
		}
	}
	return s
}

// EscapePipe escapes the pipe character so a value renders inside a Markdown
// table cell instead of starting a new column.
func EscapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
