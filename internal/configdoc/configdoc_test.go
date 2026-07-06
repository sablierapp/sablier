package configdoc_test

import (
	"testing"

	"github.com/sablierapp/sablier/internal/configdoc"
	"github.com/sablierapp/sablier/internal/docsver"
)

func TestSinceByFlag(t *testing.T) {
	since, err := configdoc.SinceByFlag("../../pkg/config")
	if err != nil {
		t.Fatalf("SinceByFlag: %v", err)
	}

	cases := map[string]string{
		"server.port":              "v1.0.0",
		"provider.name":            "v1.0.0",
		"provider.docker.strategy": "v1.11.0",
		"provider.docker.host":     docsver.NextRelease,
		"tracing.enabled":          "v1.13.0",
	}
	for flag, want := range cases {
		if got := since[flag]; got != want {
			t.Errorf("since[%q] = %q, want %q", flag, got, want)
		}
	}

	// Every parsed value must render to a non-empty label (no malformed entries).
	for flag, v := range since {
		if label, _ := docsver.Display(v); label == "" {
			t.Errorf("since[%q] = %q renders an empty label", flag, v)
		}
	}
}
