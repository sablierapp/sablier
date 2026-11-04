package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// InstanceEventsRequest holds the query parameters for the SSE events endpoint.
type InstanceEventsRequest struct {
	// Types filters which event types are streamed.
	// Repeat the parameter to subscribe to multiple types:
	//   ?types=created&types=removed
	// Omit entirely to receive all event types.
	Types []string `form:"types"`
	// Names restricts the stream to these instances.
	Names []string `form:"names"`
	// Group restricts the stream to the instances of this group.
	Group string `form:"group"`
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
// Filter by event type with the repeated `types` query parameter, and by
// instance with either `names` or `group`.
//
// @Summary      Instance events (SSE)
// @Description  Server-Sent Events stream of instance lifecycle events. Each event is `event: <type>` followed by `data: <InstanceInfo JSON>`. The stream stays open until the client disconnects. Narrow it to some instances with either `names` or `group`, never both.
// @ID           streamInstanceEvents
// @Tags         events
// @Produce      text/event-stream
// @Param        types  query  []string  false  "Event types to include (created, updated, removed, started, stopped). Repeat for multiple."
// @Param        names  query  []string  false  "Only stream events of these instances. Repeat for multiple. An 'optional:' prefix is ignored. Mutually exclusive with group."
// @Param        group  query  string    false  "Only stream events of the instances of this group, following instances that join or leave it while the stream is open. Mutually exclusive with names."
// @Success      200  {string}  string  "SSE stream (text/event-stream)"
// @Failure      400  {object}  rfc7807.Problem  "Validation error"
// @Failure      503  {object}  rfc7807.Problem  "This replica is not the leader"
// @Router       /api/events [get]
func InstanceEvents(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/events", func(c *gin.Context) {
		var req InstanceEventsRequest
		if err := c.ShouldBind(&req); err != nil {
			AbortWithProblemDetail(c, ProblemValidation(err))
			return
		}

		if len(req.Names) > 0 && req.Group != "" {
			AbortWithProblemDetail(c, ProblemValidation(errors.New("'names' and 'group' query parameters are both set, only one must be set")))
			return
		}
		names := req.Names
		if req.Group != "" {
			names = s.Sablier.Groups()[req.Group]
		}
		filter := newEventFilter(names, req.Group)

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
				if !filter.match(event) {
					continue
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

// eventFilter selects the events of the instances a subscriber asked for.
// A nil filter matches every event.
type eventFilter struct {
	// names are the requested instances, or the current members of the
	// requested group.
	names map[string]bool
	// group, when set, makes names follow the group's membership.
	group string
}

// newEventFilter returns a filter for names, or for group starting from its
// current members names.
func newEventFilter(names []string, group string) *eventFilter {
	if len(names) == 0 && group == "" {
		return nil
	}
	f := &eventFilter{names: map[string]bool{}, group: group}
	for _, name := range names {
		f.names[strings.TrimPrefix(name, sablier.OptionalPrefix)] = true
	}
	return f
}

// match reports whether event concerns the requested instances. For a group,
// it tracks membership from the events themselves: an event labelled with the
// group adds its instance, and the event that takes an instance out (removed,
// or updated without the group label) is still delivered before the instance
// is forgotten. Removal and start/stop events may not carry labels, so
// membership cannot be read from each event alone.
func (f *eventFilter) match(event sablier.InstanceEvent) bool {
	if f == nil {
		return true
	}
	name := event.Info.Name
	if f.group == "" {
		return f.names[name]
	}

	if slices.Contains(event.Info.Groups, f.group) {
		f.names[name] = true
		return true
	}
	if !f.names[name] {
		return false
	}
	if event.Type == provider.InstanceEventRemoved || event.Type == provider.InstanceEventUpdated {
		delete(f.names, name)
	}
	return true
}
