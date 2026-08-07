package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestInstanceEvents_StreamsCreatedAndRemovedEvents(t *testing.T) {
	app, router, strategy, mock := NewApiTest(t)
	InstanceEvents(router, strategy)

	eventsC := make(chan sablier.InstanceEvent, 2)
	errC := make(chan error, 1)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventCreated,
		Info: sablier.InstanceInfo{Name: "nginx", Status: sablier.InstanceStatusStarting},
	}
	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventRemoved,
		Info: sablier.InstanceInfo{Name: "nginx"},
	}
	close(eventsC)

	mock.EXPECT().
		InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{}).
		Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	r := PerformRequest(app, "GET", "/api/events")

	assert.Equal(t, http.StatusOK, r.Code)
	assert.Assert(t, strings.HasPrefix(r.Header().Get("Content-Type"), "text/event-stream"))

	body := r.Body.String()
	assert.Assert(t, strings.Contains(body, "event:created"), "expected 'event:created' in body, got: %s", body)
	assert.Assert(t, strings.Contains(body, "event:removed"), "expected 'event:removed' in body, got: %s", body)
	assert.Assert(t, strings.Contains(body, `"name":"nginx"`), "expected instance name in body")
}

func TestInstanceEvents_TypeFilterPassedToProvider(t *testing.T) {
	app, router, strategy, mock := NewApiTest(t)
	InstanceEvents(router, strategy)

	eventsC := make(chan sablier.InstanceEvent, 1)
	errC := make(chan error, 1)

	eventsC <- sablier.InstanceEvent{
		Type: provider.InstanceEventStarted,
		Info: sablier.InstanceInfo{Name: "myapp", Status: sablier.InstanceStatusReady},
	}
	close(eventsC)

	expectedOpts := provider.InstanceEventsOptions{
		Types: []provider.InstanceEventType{
			provider.InstanceEventStarted,
			provider.InstanceEventStopped,
		},
	}

	mock.EXPECT().
		InstanceEvents(gomock.Any(), expectedOpts).
		Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	r := PerformRequest(app, "GET", "/api/events?types=started&types=stopped")

	assert.Equal(t, http.StatusOK, r.Code)
	body := r.Body.String()
	assert.Assert(t, strings.Contains(body, "event:started"), "expected 'event:started' in body")
	assert.Assert(t, strings.Contains(body, `"name":"myapp"`))
}

func TestInstanceEvents_ProviderErrorClosesStream(t *testing.T) {
	app, router, strategy, mock := NewApiTest(t)
	InstanceEvents(router, strategy)

	eventsC := make(chan sablier.InstanceEvent)
	errC := make(chan error, 1)
	errC <- errors.New("provider error")
	close(eventsC)

	mock.EXPECT().
		InstanceEvents(gomock.Any(), provider.InstanceEventsOptions{}).
		Return(sablier.InstanceEventStream{Events: eventsC, Err: errC})

	r := PerformRequest(app, "GET", "/api/events")

	// Stream closes cleanly on error — response is still 200 with whatever was buffered.
	assert.Equal(t, http.StatusOK, r.Code)
}
