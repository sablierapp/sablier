package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFiles embed.FS

// Register mounts the dashboard routes on the provided router group.
func Register(group *gin.RouterGroup) {
	group.GET("", indexHandler)
	group.GET("/", indexHandler)
	group.GET("/stream", sseHandler)
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

func indexHandler(c *gin.Context) {
	data := MockData()
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_ = DashboardPage(data).Render(c.Request.Context(), c.Writer)
}

// sseHandler streams live dashboard updates as Server-Sent Events.
func sseHandler(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send initial data immediately
	sendSSEData(c)
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendSSEData(c)
			c.Writer.Flush()
		}
	}
}

func sendSSEData(c *gin.Context) {
	data := MockData()

	// Build instance payloads
	type slotPayload struct {
		Time  time.Time `json:"time"`
		State SlotState `json:"state"`
	}
	type instancePayload struct {
		Name               string        `json:"name"`
		Status             string        `json:"status"`
		Provider           string        `json:"provider"`
		Groups             []string      `json:"groups"`
		Message            string        `json:"message,omitempty"`
		SessionTTL         *int64        `json:"sessionTTL,omitempty"` // nanoseconds
		ExpiresAt          *time.Time    `json:"expiresAt,omitempty"`
		LastAccess         *time.Time    `json:"lastAccess,omitempty"`
		UptimeSlots        []slotPayload `json:"uptimeSlots"`
		SavedCO2Grams      float64       `json:"savedCO2Grams"`
		IdlePercent        float64       `json:"idlePercent"`
		TotalDowntimeHours float64       `json:"totalDowntimeHours"`
		RunningHours       string        `json:"runningHours,omitempty"`
		ReadyAfter         int64         `json:"readyAfter,omitempty"` // nanoseconds
	}

	instPayloads := make([]instancePayload, len(data.Instances))
	for i, inst := range data.Instances {
		slots := make([]slotPayload, len(inst.UptimeSlots))
		for j, s := range inst.UptimeSlots {
			slots[j] = slotPayload{Time: s.Time, State: s.State}
		}
		p := instancePayload{
			Name:               inst.Info.Name,
			Status:             string(inst.Info.Status),
			Provider:           inst.Info.Provider,
			Groups:             inst.Info.Groups,
			Message:            inst.Info.Message,
			ExpiresAt:          inst.ExpiresAt,
			LastAccess:         inst.LastAccess,
			UptimeSlots:        slots,
			SavedCO2Grams:      inst.SavedCO2Grams,
			IdlePercent:        inst.IdlePercent,
			TotalDowntimeHours: inst.TotalDowntimeHours,
			RunningHours:       inst.Info.RunningHours,
			ReadyAfter:         inst.Info.ReadyAfter.Nanoseconds(),
		}
		if inst.SessionTTL != nil {
			ns := inst.SessionTTL.Nanoseconds()
			p.SessionTTL = &ns
		}
		instPayloads[i] = p
	}

	writeSSEEvent(c, "instances", instPayloads)

	// Pending requests
	type pendingPayload struct {
		ID          string    `json:"id"`
		Names       []string  `json:"names"`
		Group       string    `json:"group,omitempty"`
		RequestedAt time.Time `json:"requestedAt"`
		Timeout     int64     `json:"timeout"` // nanoseconds
	}
	pendingPayloads := make([]pendingPayload, len(data.PendingRequests))
	for i, req := range data.PendingRequests {
		pendingPayloads[i] = pendingPayload{
			ID:          req.ID,
			Names:       req.Names,
			Group:       req.Group,
			RequestedAt: req.RequestedAt,
			Timeout:     req.Timeout.Nanoseconds(),
		}
	}
	writeSSEEvent(c, "pending", pendingPayloads)

	// Aggregate stats
	type statsPayload struct {
		Total              int     `json:"total"`
		Active             int     `json:"active"`
		Errors             int     `json:"errors"`
		Pending            int     `json:"pending"`
		TotalCO2Grams      float64 `json:"totalCO2Grams"`
		TotalDowntimeHours float64 `json:"totalDowntimeHours"`
	}
	stats := statsPayload{Total: len(data.Instances), Pending: len(data.PendingRequests)}
	for _, inst := range data.Instances {
		switch inst.Info.Status {
		case "ready", "starting":
			stats.Active++
		case "error":
			stats.Errors++
		}
		stats.TotalCO2Grams += inst.SavedCO2Grams
		stats.TotalDowntimeHours += inst.TotalDowntimeHours
	}
	writeSSEEvent(c, "stats", stats)
}

func writeSSEEvent(c *gin.Context, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(b))
}
