package dashboard

import (
	"time"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// DashboardInstance is a single instance as shown on the dashboard.
type DashboardInstance struct {
	Info sablier.InstanceInfo
	// EfficiencyPct is the percentage of the uptime window the instance was idle (0–100).
	// A value of 90 means the instance was stopped 90% of the time Sablier has been running.
	EfficiencyPct float64
	// ActiveSeconds is the total seconds the instance spent in the Ready state.
	ActiveSeconds float64
	// UptimeWindowSeconds is the total observation window in seconds (since Sablier started).
	UptimeWindowSeconds float64
}

// UptimeSlot represents a single 5-minute monitoring window.
type UptimeSlot struct {
	Time  time.Time
	State SlotState
}

// SlotState describes the state of a 5-minute slot.
type SlotState int

const (
	SlotStateUp       SlotState = iota // fully up and serving traffic
	SlotStateStarting                  // starting up
	SlotStateIdle                      // stopped / idle (resource saved)
	SlotStateError                     // error state
)

// MockPendingRequest represents an in-flight request waiting for instances.
type MockPendingRequest struct {
	ID          string
	Names       []string
	Group       string
	RequestedAt time.Time
	Timeout     time.Duration
}

// DashboardData is the full data model rendered by the dashboard.
type DashboardData struct {
	Instances      []DashboardInstance
	ProviderConfig config.Provider
	SessionConfig  config.Sessions
	GeneratedAt    time.Time
	AllGroups      []string // sorted deduplicated list of all groups across instances
}

// clockNow is used in templates (avoids importing time directly there).
func clockNow() time.Time { return time.Now() }

// MockInstance is kept for mock data generation only.
type MockInstance struct {
	Info        sablier.InstanceInfo
	SessionTTL  *time.Duration
	ExpiresAt   *time.Time
	LastAccess  *time.Time
	UptimeSlots []UptimeSlot
	IdlePercent float64
	// TotalDowntimeHours is hours the instance was stopped (estimate).
	TotalDowntimeHours float64
	// SavedCO2Grams is kept for backwards compatibility with mock data only.
	SavedCO2Grams float64
}

func MockData() struct {
	Instances       []MockInstance
	PendingRequests []MockPendingRequest
	ProviderConfig  config.Provider
	SessionConfig   config.Sessions
	GeneratedAt     time.Time
	AllGroups       []string
} {
	now := time.Now()

	ttl5m := 5 * time.Minute
	ttl10m := 10 * time.Minute
	ttl2m := 2 * time.Minute

	exp1 := now.Add(4 * time.Minute)
	exp2 := now.Add(9*time.Minute + 15*time.Second)
	exp3 := now.Add(1*time.Minute + 42*time.Second)

	readyAt1 := now.Add(-3 * time.Minute)
	readyAt2 := now.Add(-8 * time.Minute)
	lastAccess1 := now.Add(-90 * time.Second)
	lastAccess2 := now.Add(-8 * time.Minute)
	lastAccess3 := now.Add(-20 * time.Second)
	lastAccess4 := now.Add(-2 * time.Hour)
	lastAccess5 := now.Add(-18 * time.Hour)

	instances := []MockInstance{
		{
			Info: sablier.InstanceInfo{
				Name:            "code-server",
				Status:          sablier.InstanceStatusReady,
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Groups:          []string{"dev-tools"},
				Provider:        sablier.ProviderDocker,
				Enabled:         "true",
				ReadyAfter:      10 * time.Second,
				ReadyAt:         &readyAt1,
				Docker: &sablier.DockerContainerInfo{
					ID:    "a1b2c3d4e5f6",
					Image: "lscr.io/linuxserver/code-server:latest",
				},
			},
			SessionTTL:         &ttl5m,
			ExpiresAt:          &exp1,
			LastAccess:         &lastAccess1,
			UptimeSlots:        mockSlots(now, 288, 0.38),
			SavedCO2Grams:      142.5,
			IdlePercent:        62,
			TotalDowntimeHours: 104,
		},
		{
			Info: sablier.InstanceInfo{
				Name:            "jupyter-lab",
				Status:          sablier.InstanceStatusReady,
				CurrentReplicas: 1,
				DesiredReplicas: 1,
				Groups:          []string{"dev-tools", "data-science"},
				Provider:        sablier.ProviderDocker,
				Enabled:         "true",
				ReadyAt:         &readyAt2,
				Docker: &sablier.DockerContainerInfo{
					ID:    "b2c3d4e5f6a1",
					Image: "jupyter/scipy-notebook:latest",
				},
			},
			SessionTTL:         &ttl10m,
			ExpiresAt:          &exp2,
			LastAccess:         &lastAccess2,
			UptimeSlots:        mockSlots(now, 288, 0.25),
			SavedCO2Grams:      218.0,
			IdlePercent:        75,
			TotalDowntimeHours: 126,
		},
		{
			Info: sablier.InstanceInfo{
				Name:            "postgres-dev",
				Status:          sablier.InstanceStatusStarting,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Groups:          []string{"databases"},
				Provider:        sablier.ProviderDocker,
				Enabled:         "true",
				Docker: &sablier.DockerContainerInfo{
					ID:    "c3d4e5f6a1b2",
					Image: "postgres:16-alpine",
				},
			},
			SessionTTL:         &ttl2m,
			ExpiresAt:          &exp3,
			LastAccess:         &lastAccess3,
			UptimeSlots:        mockSlots(now, 288, 0.15),
			SavedCO2Grams:      305.2,
			IdlePercent:        85,
			TotalDowntimeHours: 142,
		},
		{
			Info: sablier.InstanceInfo{
				Name:            "redis-cache",
				Status:          sablier.InstanceStatusStopped,
				CurrentReplicas: 0,
				DesiredReplicas: 0,
				Groups:          []string{"databases"},
				Provider:        sablier.ProviderDocker,
				Enabled:         "true",
				Docker: &sablier.DockerContainerInfo{
					ID:    "d4e5f6a1b2c3",
					Image: "redis:7-alpine",
				},
			},
			SessionTTL:         nil,
			ExpiresAt:          nil,
			LastAccess:         &lastAccess4,
			UptimeSlots:        mockSlots(now, 288, 0.08),
			SavedCO2Grams:      389.1,
			IdlePercent:        92,
			TotalDowntimeHours: 154,
		},
		{
			Info: sablier.InstanceInfo{
				Name:            "grafana",
				Status:          sablier.InstanceStatusStopped,
				CurrentReplicas: 0,
				DesiredReplicas: 0,
				Groups:          []string{"monitoring"},
				Provider:        sablier.ProviderDocker,
				Enabled:         "true",
				RunningHours:    "09:00-17:00",
				Docker: &sablier.DockerContainerInfo{
					ID:    "e5f6a1b2c3d4",
					Image: "grafana/grafana:latest",
				},
			},
			SessionTTL:         nil,
			ExpiresAt:          nil,
			LastAccess:         &lastAccess5,
			UptimeSlots:        mockSlots(now, 288, 0.30),
			SavedCO2Grams:      260.0,
			IdlePercent:        70,
			TotalDowntimeHours: 117,
		},
		{
			Info: sablier.InstanceInfo{
				Name:            "api-staging",
				Status:          sablier.InstanceStatusError,
				CurrentReplicas: 0,
				DesiredReplicas: 1,
				Groups:          []string{"staging"},
				Provider:        sablier.ProviderKubernetes,
				Enabled:         "true",
				Message:         "OOMKilled: container exceeded memory limit",
				Kubernetes: &sablier.KubernetesWorkloadInfo{
					Namespace: "staging",
					Kind:      "Deployment",
					Image:     "myorg/api:v2.1.0",
				},
			},
			SessionTTL:         &ttl5m,
			ExpiresAt:          &exp1,
			LastAccess:         &lastAccess3,
			UptimeSlots:        mockSlots(now, 288, 0.20),
			SavedCO2Grams:      91.3,
			IdlePercent:        80,
			TotalDowntimeHours: 134,
		},
	}

	pendingRequests := []MockPendingRequest{
		{
			ID:          "req-7f3a2e1b",
			Names:       []string{"postgres-dev"},
			RequestedAt: now.Add(-12 * time.Second),
			Timeout:     30 * time.Second,
		},
		{
			ID:          "req-9c4b5d6e",
			Group:       "data-science",
			RequestedAt: now.Add(-3 * time.Second),
			Timeout:     30 * time.Second,
		},
	}

	providerConf := config.Provider{
		Name:                      "docker",
		AutoStopOnStartup:         true,
		AutoStopExternallyStarted: false,
		RejectUnlabeledRequests:   false,
		VerifyEnabledOnExpiration: true,
		Docker:                    config.Docker{Strategy: "stop"},
	}

	sessionConf := config.Sessions{
		DefaultDuration:    5 * time.Minute,
		ExpirationInterval: 20 * time.Second,
	}

	return struct {
		Instances       []MockInstance
		PendingRequests []MockPendingRequest
		ProviderConfig  config.Provider
		SessionConfig   config.Sessions
		GeneratedAt     time.Time
		AllGroups       []string
	}{
		Instances:       instances,
		PendingRequests: pendingRequests,
		ProviderConfig:  providerConf,
		SessionConfig:   sessionConf,
		GeneratedAt:     now,
		AllGroups:       collectGroups(instances),
	}
}

func collectGroups(instances []MockInstance) []string {
	seen := map[string]bool{}
	var out []string
	for _, inst := range instances {
		for _, g := range inst.Info.Groups {
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// mockSlots generates 288 5-minute slots (24h) with a realistic pattern.
// upFraction is the target fraction of slots that are "up".
func mockSlots(now time.Time, count int, upFraction float64) []UptimeSlot {
	slots := make([]UptimeSlot, count)
	// Use a deterministic pseudo-pattern based on time-of-day
	for i := 0; i < count; i++ {
		t := now.Add(-time.Duration(count-i) * 5 * time.Minute)
		hour := t.Hour()
		// During working hours (8-18) more likely to be up
		inWorkHours := hour >= 8 && hour < 18
		var threshold float64
		if inWorkHours {
			threshold = upFraction * 2.2
		} else {
			threshold = upFraction * 0.3
		}
		if threshold > 1 {
			threshold = 1
		}
		// Deterministic: use index modulo to avoid random
		var state SlotState
		frac := float64((i*37+13)%100) / 100.0
		if frac < threshold {
			if (i*7)%20 == 0 {
				state = SlotStateStarting
			} else {
				state = SlotStateUp
			}
		} else {
			if (i*11)%50 == 0 && i > 0 {
				state = SlotStateError
			} else {
				state = SlotStateIdle
			}
		}
		slots[i] = UptimeSlot{Time: t, State: state}
	}
	return slots
}
