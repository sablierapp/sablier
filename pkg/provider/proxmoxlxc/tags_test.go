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

func TestExtractGroups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "has group tag",
			tags: []string{"sablier", "sablier-group-web"},
			want: []string{"web"},
		},
		{
			name: "no group tag defaults to default",
			tags: []string{"sablier"},
			want: []string{"default"},
		},
		{
			name: "empty group prefix is ignored",
			tags: []string{"sablier", "sablier-group-"},
			want: []string{"default"},
		},
		{
			name: "multiple group tags returns all",
			tags: []string{"sablier-group-first", "sablier-group-second"},
			want: []string{"first", "second"},
		},
		{
			name: "empty tags",
			tags: nil,
			want: []string{"default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractGroups(tt.tags)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
