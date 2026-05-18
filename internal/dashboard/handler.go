package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

//go:embed static
var staticFiles embed.FS

// SnapshotProvider is the subset of *sablier.Sablier used by the dashboard.
type SnapshotProvider interface {
	SnapshotSessions(ctx context.Context) ([]sablier.InstanceInfo, error)
	Groups() map[string][]string
}

// Handler is the dashboard HTTP handler, wired with real dependencies.
type Handler struct {
	Sablier        SnapshotProvider
	Metrics        metrics.Recorder
	ProviderConfig config.Provider
	SessionConfig  config.Sessions
}

// Register mounts the dashboard routes on the provided router group.
func Register(group *gin.RouterGroup, h *Handler) {
	group.GET("", h.indexHandler)
	group.GET("/", h.indexHandler)
	group.GET("/stream", h.sseHandler)
	group.GET("/static/:file", staticHandler)
}

func staticHandler(c *gin.Context) {
	file := c.Param("file")
	data, err := staticFiles.ReadFile("static/" + file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ct := "application/octet-stream"
	switch {
	case len(file) > 3 && file[len(file)-3:] == ".js":
		ct = "application/javascript"
	case len(file) > 4 && file[len(file)-4:] == ".css":
		ct = "text/css"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(data)
}

func (h *Handler) indexHandler(c *gin.Context) {
	data := h.buildDashboardData(c.Request.Context())
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_ = DashboardPage(data).Render(c.Request.Context(), c.Writer)
}

// sseHandler streams live dashboard updates as Server-Sent Events.
func (h *Handler) sseHandler(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send initial data immediately
	h.sendSSEData(c)
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sendSSEData(c)
			c.Writer.Flush()
		}
	}
}

func (h *Handler) buildDashboardData(ctx context.Context) DashboardData {
	now := time.Now()

	// Fetch sessions from the real store + provider.
	var instances []DashboardInstance
	if h.Sablier != nil {
		infos, err := h.Sablier.SnapshotSessions(ctx)
		if err == nil {
			// Get active-seconds from metrics if available.
			var activeSeconds map[string]float64
			if pr, ok := h.Metrics.(*metrics.PromRecorder); ok {
				activeSeconds = pr.ActiveSecondsSnapshot()
				_ = pr.StartedAt // keep reference
			}
			uptime := now.Sub(metricsStartedAt(h.Metrics))

			for _, info := range infos {
				d := DashboardInstance{Info: info}
				if activeSeconds != nil {
					secs := activeSeconds[info.Name]
					total := uptime.Seconds()
					if total > 0 {
						d.EfficiencyPct = (1 - secs/total) * 100
						if d.EfficiencyPct < 0 {
							d.EfficiencyPct = 0
						}
						if d.EfficiencyPct > 100 {
							d.EfficiencyPct = 100
						}
					}
					d.ActiveSeconds = secs
					d.UptimeWindowSeconds = total
				}
				instances = append(instances, d)
			}
		}
	}

	allGroups := collectAllGroups(instances)

	return DashboardData{
		Instances:      instances,
		ProviderConfig: h.ProviderConfig,
		SessionConfig:  h.SessionConfig,
		GeneratedAt:    now,
		AllGroups:      allGroups,
	}
}

// metricsStartedAt returns when the PromRecorder was created (for efficiency window).
func metricsStartedAt(rec metrics.Recorder) time.Time {
	if pr, ok := rec.(*metrics.PromRecorder); ok {
		return pr.StartedAt
	}
	return time.Now()
}

func (h *Handler) sendSSEData(c *gin.Context) {
	data := h.buildDashboardData(c.Request.Context())

	// Build instance payloads
	type instancePayload struct {
		Name                string                          `json:"name"`
		Status              string                          `json:"status"`
		Provider            string                          `json:"provider"`
		Groups              []string                        `json:"groups"`
		Message             string                          `json:"message,omitempty"`
		ExpiresInSeconds    *float64                        `json:"expiresInSeconds,omitempty"`
		EfficiencyPct       float64                         `json:"efficiencyPct"`
		ActiveSeconds       float64                         `json:"activeSeconds"`
		UptimeWindowSeconds float64                         `json:"uptimeWindowSeconds"`
		RunningHours        string                          `json:"runningHours,omitempty"`
		ReadyAfter          int64                           `json:"readyAfter,omitempty"` // nanoseconds
		Docker              *sablier.DockerContainerInfo    `json:"docker,omitempty"`
		Kubernetes          *sablier.KubernetesWorkloadInfo `json:"kubernetes,omitempty"`
	}

	instPayloads := make([]instancePayload, len(data.Instances))
	var startingPayloads []instancePayload
	for i, inst := range data.Instances {
		p := instancePayload{
			Name:                inst.Info.Name,
			Status:              string(inst.Info.Status),
			Provider:            inst.Info.Provider,
			Groups:              inst.Info.Groups,
			Message:             inst.Info.Message,
			EfficiencyPct:       inst.EfficiencyPct,
			ActiveSeconds:       inst.ActiveSeconds,
			UptimeWindowSeconds: inst.UptimeWindowSeconds,
			RunningHours:        inst.Info.RunningHours,
			ReadyAfter:          inst.Info.ReadyAfter.Nanoseconds(),
			Docker:              inst.Info.Docker,
			Kubernetes:          inst.Info.Kubernetes,
		}
		if inst.Info.ExpiresAt != nil {
			if remaining := time.Until(*inst.Info.ExpiresAt).Seconds(); remaining > 0 {
				p.ExpiresInSeconds = &remaining
			}
		}
		instPayloads[i] = p
		if inst.Info.Status == sablier.InstanceStatusStarting {
			startingPayloads = append(startingPayloads, p)
		}
	}
	writeSSEEvent(c, "instances", instPayloads)

	// Aggregate stats
	type statsPayload struct {
		Total             int     `json:"total"`
		Active            int     `json:"active"`
		Starting          int     `json:"starting"`
		Errors            int     `json:"errors"`
		Pending           int     `json:"pending"`
		OverallEfficiency float64 `json:"overallEfficiency"`
	}
	stats := statsPayload{Total: len(data.Instances)}
	var effSum float64
	effCount := 0
	for _, inst := range data.Instances {
		switch inst.Info.Status {
		case sablier.InstanceStatusReady:
			stats.Active++
		case sablier.InstanceStatusStarting:
			stats.Active++
			stats.Starting++
		case sablier.InstanceStatusError:
			stats.Errors++
		}
		if inst.UptimeWindowSeconds > 0 {
			effSum += inst.EfficiencyPct
			effCount++
		}
	}
	if effCount > 0 {
		stats.OverallEfficiency = effSum / float64(effCount)
	}
	writeSSEEvent(c, "stats", stats)

	// Emit starting instances with full info so the client can name them.
	if startingPayloads == nil {
		startingPayloads = []instancePayload{}
	}
	writeSSEEvent(c, "starting", startingPayloads)

	// Send the current distinct groups so the client can update the filter pills.
	groups := data.AllGroups
	if groups == nil {
		groups = []string{}
	}
	writeSSEEvent(c, "groups", groups)
}

func writeSSEEvent(c *gin.Context, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(b))
}

func collectAllGroups(instances []DashboardInstance) []string {
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

// Fallback to mock data for the initial HTML render when no real provider is configured.
// This function is intentionally kept for development/demo purposes.
func buildFallbackData() DashboardData {
	m := MockData()
	instances := make([]DashboardInstance, len(m.Instances))
	for i, mi := range m.Instances {
		instances[i] = DashboardInstance{
			Info:                mi.Info,
			EfficiencyPct:       mi.IdlePercent,
			ActiveSeconds:       mi.TotalDowntimeHours * 3600 * (1 - mi.IdlePercent/100),
			UptimeWindowSeconds: mi.TotalDowntimeHours * 3600,
		}
	}
	return DashboardData{
		Instances:      instances,
		ProviderConfig: m.ProviderConfig,
		SessionConfig:  m.SessionConfig,
		GeneratedAt:    m.GeneratedAt,
		AllGroups:      m.AllGroups,
	}
}

// Ensure the Handler works with provider.InstanceListOptions (keep import alive).
var _ provider.InstanceListOptions = provider.InstanceListOptions{}
