package docsmd

import "testing"

func TestFirstSentence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no sentence end", "a short usage line", "a short usage line"},
		{"first of two", "First sentence. Second sentence.", "First sentence."},
		{"newline is flattened", "First line\ncontinues. Second.", "First line continues."},
		{"abbreviation does not split", "Run on a schedule, e.g. daily. Then stops.", "Run on a schedule, e.g. daily."},
		{"abbreviation with paren prefix", "See the value (e.g. 5m) here. Next.", "See the value (e.g. 5m) here."},
		{"trailing period without space", "Ends with a period.", "Ends with a period."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstSentence(tt.in); got != tt.want {
				t.Fatalf("FirstSentence(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapePipe(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"no pipe", "no pipe"},
		{"a | b", "a \\| b"},
		{"a | b | c", "a \\| b \\| c"},
	}
	for _, tt := range tests {
		if got := EscapePipe(tt.in); got != tt.want {
			t.Fatalf("EscapePipe(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
