package proxmoxlxc

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single tag",
			input: "sablier",
			want:  []string{"sablier"},
		},
		{
			name:  "multiple tags",
			input: "sablier;sablier-group-web;production",
			want:  []string{"sablier", "sablier-group-web", "production"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseTags(tt.input)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestHasSablierTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{
			name: "has sablier tag",
			tags: []string{"sablier", "other"},
			want: true,
		},
		{
			name: "no sablier tag",
			tags: []string{"other", "tags"},
			want: false,
		},
		{
			name: "empty tags",
			tags: nil,
			want: false,
		},
		{
			name: "sablier-group is not sablier",
			tags: []string{"sablier-group-web"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasSablierTag(tt.tags)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestExtractGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "has group tag",
			tags: []string{"sablier", "sablier-group-web"},
			want: "web",
		},
		{
			name: "no group tag defaults to default",
			tags: []string{"sablier"},
			want: "default",
		},
		{
			name: "empty group prefix is ignored",
			tags: []string{"sablier", "sablier-group-"},
			want: "default",
		},
		{
			name: "multiple group tags uses first",
			tags: []string{"sablier-group-first", "sablier-group-second"},
			want: "first",
		},
		{
			name: "empty tags",
			tags: nil,
			want: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractGroup(tt.tags)
			assert.Equal(t, got, tt.want)
		})
	}
}
