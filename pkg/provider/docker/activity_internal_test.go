package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestCgroupPath(t *testing.T) {
	tests := []struct {
		name, driver, parent, id, want string
	}{
		{"systemd default parent", "systemd", "", "abc", "/sys/fs/cgroup/system.slice/docker-abc.scope"},
		{"systemd custom slice", "systemd", "machine-sablier.slice", "abc", "/sys/fs/cgroup/machine.slice/machine-sablier.slice/docker-abc.scope"},
		{"cgroupfs default parent", "cgroupfs", "", "abc", "/sys/fs/cgroup/docker/abc"},
		{"cgroupfs custom parent", "cgroupfs", "/custom/parent", "abc", "/sys/fs/cgroup/custom/parent/abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, cgroupPath(tt.driver, tt.parent, tt.id), tt.want)
		})
	}
}

func TestContainerToInstance_Activity(t *testing.T) {
	c := container.Summary{
		Names:  []string{"/web"},
		Labels: map[string]string{sablier.LabelEnable: "true", sablier.LabelActivityEnable: "true"},
	}
	assert.Assert(t, containerToInstance(c).Activity)

	c.Labels[sablier.LabelActivityEnable] = "false"
	assert.Assert(t, !containerToInstance(c).Activity)
}
