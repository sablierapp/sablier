package systemd

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestLabelsFromUnitFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{name: "no section", content: "[Service]\nExecStart=/bin/true\n", want: nil},
		{name: "empty section", content: "[X-Sablier]\n", want: nil},
		{
			name:    "enable and group",
			content: "[X-Sablier]\nEnable=true\nGroup=team-a\n",
			want:    map[string]string{"sablier.enable": "true", "sablier.group": "team-a"},
		},
		{
			name:    "canonical labels",
			content: "[X-Sablier]\nReadyAfter=30s\nRunningHours=09:00-18:00\nAntiAffinity=streaming\n",
			want:    map[string]string{"sablier.ready-after": "30s", "sablier.running-hours": "09:00-18:00", "sablier.anti-affinity": "streaming"},
		},
		{
			name:    "scale labels",
			content: "[X-Sablier]\nIdleReplicas=1\nIdleCPU=0.25\nActiveCPU=0.5\nIdleBlkioDeviceReadBps=/dev/sda:10m\n",
			want:    map[string]string{"sablier.idle.replicas": "1", "sablier.idle.cpu": "0.25", "sablier.active.cpu": "0.5", "sablier.idle.blkio-device-read-bps": "/dev/sda:10m"},
		},
		{name: "unknown keys are dropped", content: "[X-Sablier]\nFoo=bar\n", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := labelsFromUnitFile(writeUnitFile(t, tt.content))
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestLabelsFromUnitFile_CoversAllKeys(t *testing.T) {
	want := map[string]string{
		"Enable":                     "sablier.enable",
		"Group":                      "sablier.group",
		"ReadyAfter":                 "sablier.ready-after",
		"ReadyOnStart":               "sablier.ready-on-start",
		"RunningHours":               "sablier.running-hours",
		"RunningDays":                "sablier.running-days",
		"AntiAffinity":               "sablier.anti-affinity",
		"IdleReplicas":               "sablier.idle.replicas",
		"IdleCPU":                    "sablier.idle.cpu",
		"IdleMemory":                 "sablier.idle.memory",
		"ActiveReplicas":             "sablier.active.replicas",
		"ActiveCPU":                  "sablier.active.cpu",
		"ActiveMemory":               "sablier.active.memory",
		"IdleBlkioWeight":            "sablier.idle.blkio-weight",
		"ActiveBlkioWeight":          "sablier.active.blkio-weight",
		"IdleBlkioWeightDevice":      "sablier.idle.blkio-weight-device",
		"ActiveBlkioWeightDevice":    "sablier.active.blkio-weight-device",
		"IdleBlkioDeviceReadBps":     "sablier.idle.blkio-device-read-bps",
		"ActiveBlkioDeviceReadBps":   "sablier.active.blkio-device-read-bps",
		"IdleBlkioDeviceWriteBps":    "sablier.idle.blkio-device-write-bps",
		"ActiveBlkioDeviceWriteBps":  "sablier.active.blkio-device-write-bps",
		"IdleBlkioDeviceReadIOps":    "sablier.idle.blkio-device-read-iops",
		"ActiveBlkioDeviceReadIOps":  "sablier.active.blkio-device-read-iops",
		"IdleBlkioDeviceWriteIOps":   "sablier.idle.blkio-device-write-iops",
		"ActiveBlkioDeviceWriteIOps": "sablier.active.blkio-device-write-iops",
	}
	assert.DeepEqual(t, bareKeyLabels, want)

	for key, label := range want {
		t.Run(key, func(t *testing.T) {
			got, err := labelsFromUnitFile(writeUnitFile(t, "[X-Sablier]\n"+key+"=v\n"))
			assert.NilError(t, err)
			assert.DeepEqual(t, got, map[string]string{label: "v"})
		})
	}
}
