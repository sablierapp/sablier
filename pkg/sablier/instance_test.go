package sablier_test

import (
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestInstanceInfo_IsReady_NoReadyAfter(t *testing.T) {
	info := sablier.InstanceInfo{Status: sablier.InstanceStatusReady}
	assert.Assert(t, info.IsReady())
}

func TestInstanceInfo_IsReady_NotReadyStatus(t *testing.T) {
	for _, status := range []sablier.InstanceStatus{
		sablier.InstanceStatusStopped,
		sablier.InstanceStatusStarting,
		sablier.InstanceStatusError,
	} {
		info := sablier.InstanceInfo{Status: status}
		assert.Assert(t, !info.IsReady(), "expected not ready for status %q", status)
	}
}

func TestInstanceInfo_IsReady_ReadyAfter_WithinGracePeriod(t *testing.T) {
	now := time.Now()
	info := sablier.InstanceInfo{
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: time.Hour,
		ReadyAt:    &now,
	}
	// Stamped just now — grace period has not elapsed.
	assert.Assert(t, !info.IsReady())
}

func TestInstanceInfo_IsReady_ReadyAfter_GracePeriodElapsed(t *testing.T) {
	past := time.Now().Add(-2 * time.Second)
	info := sablier.InstanceInfo{
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: time.Second,
		ReadyAt:    &past,
	}
	// ReadyAt was 2s ago and ReadyAfter is 1s → grace period elapsed.
	assert.Assert(t, info.IsReady())
}

func TestInstanceInfo_IsReady_ReadyAfter_NilReadyAt(t *testing.T) {
	// ReadyAfter is set but ReadyAt was never stamped — treat as ready immediately.
	info := sablier.InstanceInfo{
		Status:     sablier.InstanceStatusReady,
		ReadyAfter: time.Hour,
		ReadyAt:    nil,
	}
	assert.Assert(t, info.IsReady())
}

func TestPopulateEnabledAndGroup_ReadyAfter(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		wantAfter time.Duration
	}{

		{
			name:      "label absent",
			labels:    map[string]string{"sablier.enable": "true"},
			wantAfter: 0,
		},
		{
			name:      "label set to 30s",
			labels:    map[string]string{"sablier.enable": "true", "sablier.ready-after": "30s"},
			wantAfter: 30 * time.Second,
		},
		{
			name:      "label set to 1m30s",
			labels:    map[string]string{"sablier.enable": "true", "sablier.ready-after": "1m30s"},
			wantAfter: 90 * time.Second,
		},
		{
			name:      "invalid label value is ignored",
			labels:    map[string]string{"sablier.enable": "true", "sablier.ready-after": "not-a-duration"},
			wantAfter: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info sablier.InstanceInfo
			sablier.PopulateEnabledAndGroup(&info, tt.labels)
			assert.Equal(t, info.ReadyAfter, tt.wantAfter)
		})
	}
}

func TestParseGroups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		label string
		want  []string
	}{
		{
			name:  "empty label defaults to default",
			label: "",
			want:  []string{"default"},
		},
		{
			name:  "single group",
			label: "web",
			want:  []string{"web"},
		},
		{
			name:  "two groups comma-separated",
			label: "team-a,team-b",
			want:  []string{"team-a", "team-b"},
		},
		{
			name:  "three groups",
			label: "frontend,backend,shared",
			want:  []string{"frontend", "backend", "shared"},
		},
		{
			name:  "spaces around commas are trimmed",
			label: "web , api , db",
			want:  []string{"web", "api", "db"},
		},
		{
			name:  "duplicate groups are deduplicated",
			label: "web,web,api",
			want:  []string{"web", "api"},
		},
		{
			name:  "only commas defaults to default",
			label: ",,",
			want:  []string{"default"},
		},
		{
			name:  "mixed empty segments are ignored",
			label: "web,,api",
			want:  []string{"web", "api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sablier.ParseGroups(tt.label)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestPopulateEnabledAndGroup_Groups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name:   "single group",
			labels: map[string]string{"sablier.enable": "true", "sablier.group": "myapp"},
			want:   []string{"myapp"},
		},
		{
			name:   "multiple groups comma-separated",
			labels: map[string]string{"sablier.enable": "true", "sablier.group": "team-a,team-b"},
			want:   []string{"team-a", "team-b"},
		},
		{
			name:   "no group label defaults to default",
			labels: map[string]string{"sablier.enable": "true"},
			want:   []string{"default"},
		},
		{
			name:   "not enabled produces no groups",
			labels: map[string]string{"sablier.enable": "false", "sablier.group": "web"},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var info sablier.InstanceInfo
			sablier.PopulateEnabledAndGroup(&info, tt.labels)
			assert.DeepEqual(t, info.Groups, tt.want)
		})
	}
}

func TestPopulateEnabledAndGroup_RunningHours(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "label absent",
			labels:   map[string]string{"sablier.enable": "true"},
			expected: "",
		},
		{
			name:     "valid running-hours",
			labels:   map[string]string{"sablier.enable": "true", "sablier.running-hours": "09:00-17:00"},
			expected: "09:00-17:00",
		},
		{
			name:     "invalid running-hours is ignored",
			labels:   map[string]string{"sablier.enable": "true", "sablier.running-hours": "invalid"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info sablier.InstanceInfo
			sablier.PopulateEnabledAndGroup(&info, tt.labels)
			assert.Equal(t, info.RunningHours, tt.expected)
		})
	}
}
