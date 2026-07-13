package docsver

import "testing"

func TestDisplay(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantLabel string
		wantLink  string
	}{
		{"empty", "", "", ""},
		{"next release", NextRelease, "Next release", ""},
		{"tagged", "v1.13.0", "v1.13.0", releaseBaseURL + "v1.13.0"},
		{"no v prefix is verbatim", "1.13.0", "1.13.0", ""},
		{"branch name verbatim", "main", "main", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, link := Display(tt.in)
			if label != tt.wantLabel || link != tt.wantLink {
				t.Fatalf("Display(%q) = (%q, %q), want (%q, %q)", tt.in, label, link, tt.wantLabel, tt.wantLink)
			}
		})
	}
}

func TestSinceBadge(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"tagged links to release", "v1.13.0", `{{< badge content="Since v1.13.0" link="https://github.com/sablierapp/sablier/releases/tag/v1.13.0" >}}`},
		{"next release", NextRelease, `{{< badge content="Next release" >}}`},
		{"verbatim unlinked", "1.13.0", `{{< badge content="Since 1.13.0" >}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SinceBadge(tt.in); got != tt.want {
				t.Fatalf("SinceBadge(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
