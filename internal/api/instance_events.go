package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/provider"
)

// InstanceEventsRequest holds the query parameters for the SSE events endpoint.
type InstanceEventsRequest struct {
	// Types filters which event types are streamed.
	// Repeat the parameter to subscribe to multiple types:
	//   ?types=created&types=removed
	// Omit entirely to receive all event types.
	Types []string `form:"types"`
}

// InstanceEvents registers GET /api/events.
//
// The endpoint streams instance lifecycle events as Server-Sent Events.
// Each SSE message carries:
//
//	event: <type>          (created | updated | removed | started | stopped)
//	data:  <json>          (InstanceInfo as JSON)
//
// The stream stays open until the client disconnects or the server shuts down.
// Filter by event type with the repeated `types` query parameter.
func InstanceEvents(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/events", func(c *gin.Context) {
		var req InstanceEventsRequest
		if err := c.ShouldBind(&req); err != nil {
			AbortWithProblemDetail(c, ProblemValidation(err))
			return
		}

		opts := provider.InstanceEventsOptions{}
		for _, t := range req.Types {
			opts.Types = append(opts.Types, provider.InstanceEventType(t))
		}

		stream := s.Sablier.InstanceEvents(c.Request.Context(), opts)

		c.Header("X-Accel-Buffering", "no")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)

		// Get the flusher for real HTTP connections (absent in unit tests).
		flusher, canFlush := c.Writer.(http.Flusher)

		for {
			select {
			case event, ok := <-stream.Events:
				if !ok {
					return
				}
				c.SSEvent(string(event.Type), event.Info)
				if canFlush {
					flusher.Flush()
				}

			case _, ok := <-stream.Err:
				if !ok {
					// nil / closed error channel — keep going.
					continue
				}
				return // terminal error from the provider

			case <-c.Request.Context().Done():
				return
			}
		}
	})
}
