package api

import (
	"encoding/json"
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

// streamEvents serves /api/events with the given query, feeding events to the
// handler, and returns the names of the instances that were streamed.
func streamEvents(t *testing.T, query string, groups map[string][]string, events ...sablier.InstanceEvent) (int, []string) {
	t.Helper()
	app, router, strategy, mock := NewApiTest(t)
	InstanceEvents(router, strategy)

	eventsC := make(chan sablier.InstanceEvent, len(events))
	for _, e := range events {
		eventsC <- e
	}
	close(eventsC)
	mock.EXPECT().Groups().Return(groups).AnyTimes()
	mock.EXPECT().InstanceEvents(gomock.Any(), gomock.Any()).
		Return(sablier.InstanceEventStream{Events: eventsC, Err: make(chan error)}).AnyTimes()

	r := PerformRequest(app, "GET", "/api/events"+query)

	var names []string
	for _, line := range strings.Split(r.Body.String(), "\n") {
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		var info sablier.InstanceInfo
		assert.NilError(t, json.Unmarshal([]byte(data), &info))
		names = append(names, info.Name)
	}
	return r.Code, names
}

func event(t provider.InstanceEventType, name string, groups ...string) sablier.InstanceEvent {
	return sablier.InstanceEvent{Type: t, Info: sablier.InstanceInfo{Name: name, Groups: groups}}
}

func TestInstanceEvents_NamesFilter(t *testing.T) {
	code, names := streamEvents(t, "?names=web&names=optional:db", nil,
		event(provider.InstanceEventStarted, "web"),
		event(provider.InstanceEventStarted, "other"),
		event(provider.InstanceEventStopped, "db"),
	)

	assert.Equal(t, http.StatusOK, code)
	assert.DeepEqual(t, []string{"web", "db"}, names)
}

func TestInstanceEvents_GroupFilterUsesCurrentMembers(t *testing.T) {
	// Started/stopped events need not carry the group labels: members known
	// when the subscription starts still match.
	code, names := streamEvents(t, "?group=demo", map[string][]string{"demo": {"web"}, "other": {"api"}},
		event(provider.InstanceEventStarted, "web"),
		event(provider.InstanceEventStarted, "api"),
	)

	assert.Equal(t, http.StatusOK, code)
	assert.DeepEqual(t, []string{"web"}, names)
}

func TestInstanceEvents_GroupFilterFollowsMembership(t *testing.T) {
	_, names := streamEvents(t, "?group=demo", map[string][]string{"demo": {"web", "db"}},
		// A new instance joins the group, then starts without labels.
		event(provider.InstanceEventCreated, "api", "demo"),
		event(provider.InstanceEventStarted, "api"),
		// A removed instance carries no labels: its removal is still
		// delivered, and it is forgotten afterwards.
		event(provider.InstanceEventRemoved, "web"),
		event(provider.InstanceEventStarted, "web"),
		// An instance relabelled out of the group: the update is
		// delivered, and it is forgotten afterwards.
		event(provider.InstanceEventUpdated, "db", "other"),
		event(provider.InstanceEventStarted, "db"),
	)

	assert.DeepEqual(t, []string{"api", "api", "web", "db"}, names)
}

func TestInstanceEvents_NamesAndGroupAreExclusive(t *testing.T) {
	code, _ := streamEvents(t, "?names=web&group=demo", nil)

	assert.Equal(t, http.StatusBadRequest, code)
}
