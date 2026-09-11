package nomad_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
)

// scaleCall records one call to the Nomad scale endpoint.
type scaleCall struct {
	JobID   string
	Group   string
	Count   int64
	Message string
}

// mockNomad is a minimal in-memory Nomad HTTP API: enough of the jobs,
// allocations, scale, register and event stream endpoints for the provider.
type mockNomad struct {
	t   *testing.T
	srv *httptest.Server

	mu          sync.Mutex
	index       uint64
	jobs        map[string]*api.Job
	allocs      map[string][]*api.AllocationListStub
	scaleCalls  []scaleCall
	registered  []*api.Job
	infoCalls   map[string]int // job ID -> GET /v1/job/:id count
	namespaces  map[string]int // namespace query parameter -> count
	tokens      map[string]int // X-Nomad-Token header -> count
	listFail    bool
	streamFail  bool
	streams     map[int]chan []byte
	nextStream  int
	streamOpens atomic.Int32
}

func newMockNomad(t *testing.T) *mockNomad {
	t.Helper()
	m := &mockNomad{
		t:          t,
		index:      10,
		jobs:       make(map[string]*api.Job),
		allocs:     make(map[string][]*api.AllocationListStub),
		infoCalls:  make(map[string]int),
		namespaces: make(map[string]int),
		tokens:     make(map[string]int),
		streams:    make(map[int]chan []byte),
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(func() {
		m.closeStreams()
		m.srv.Close()
	})
	return m
}

// client returns an API client for the mock. The namespace is left unset so
// the namespace sent by the provider itself is observable.
func (m *mockNomad) client(t *testing.T) *api.Client {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = m.srv.URL
	cfg.SecretID = "test-token"
	cfg.Namespace = ""
	client, err := api.NewClient(cfg)
	if err != nil {
		t.Fatalf("cannot create nomad client: %v", err)
	}
	return client
}

func (m *mockNomad) provider(t *testing.T) *nomad.Provider {
	t.Helper()
	p, err := nomad.NewForTest(t.Context(), m.client(t), "default", slogt.New(t), 10*time.Millisecond)
	if err != nil {
		t.Fatalf("cannot create provider: %v", err)
	}
	return p
}

func (m *mockNomad) addJob(job *api.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeLocked(job)
}

func (m *mockNomad) storeLocked(job *api.Job) {
	m.index++
	job.ModifyIndex = new(m.index)
	job.JobModifyIndex = new(m.index)
	if job.Namespace == nil {
		job.Namespace = new("default")
	}
	m.jobs[*job.ID] = job
}

func (m *mockNomad) setAllocs(jobID string, allocs ...*api.AllocationListStub) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocs[jobID] = allocs
}

func (m *mockNomad) setListFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listFail = fail
}

func (m *mockNomad) setStreamFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamFail = fail
}

func (m *mockNomad) getScaleCalls() []scaleCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]scaleCall(nil), m.scaleCalls...)
}

func (m *mockNomad) getRegistered() []*api.Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*api.Job(nil), m.registered...)
}

func (m *mockNomad) getInfoCalls(jobID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.infoCalls[jobID]
}

func (m *mockNomad) getNamespaces() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int, len(m.namespaces))
	for k, v := range m.namespaces {
		out[k] = v
	}
	return out
}

func (m *mockNomad) getTokens() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int, len(m.tokens))
	for k, v := range m.tokens {
		out[k] = v
	}
	return out
}

// pushJobEvent stores the job (or deletes it when purged) and broadcasts a
// Job topic event to every connected stream, the way Nomad does.
func (m *mockNomad) pushJobEvent(eventType string, job *api.Job, deleted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if deleted {
		m.index++
		delete(m.jobs, *job.ID)
	} else {
		m.storeLocked(job)
	}
	payload := map[string]any{"Job": job}
	if deleted {
		payload["Deleted"] = true
	}
	batch := api.Events{
		Index: m.index,
		Events: []api.Event{{
			Topic:   api.TopicJob,
			Type:    eventType,
			Key:     *job.ID,
			Index:   m.index,
			Payload: payload,
		}},
	}
	b, err := json.Marshal(batch)
	if err != nil {
		m.t.Errorf("cannot marshal event: %v", err)
		return
	}
	for _, ch := range m.streams {
		select {
		case ch <- b:
		default:
			m.t.Errorf("event stream buffer full")
		}
	}
}

// closeStreams drops every connected event stream, like a server restart.
func (m *mockNomad) closeStreams() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ch := range m.streams {
		close(ch)
		delete(m.streams, id)
	}
}

func (m *mockNomad) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.namespaces[r.URL.Query().Get("namespace")]++
	m.tokens[r.Header.Get("X-Nomad-Token")]++
	m.mu.Unlock()

	path := r.URL.EscapedPath()
	switch {
	case path == "/v1/jobs" && r.Method == http.MethodGet:
		m.handleList(w)
	case path == "/v1/jobs" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		m.handleRegister(w, r)
	case path == "/v1/event/stream":
		m.handleStream(w, r)
	case strings.HasPrefix(path, "/v1/job/"):
		m.handleJob(w, r, strings.TrimPrefix(path, "/v1/job/"))
	default:
		http.NotFound(w, r)
	}
}

func (m *mockNomad) handleList(w http.ResponseWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listFail {
		http.Error(w, "nomad unavailable", http.StatusInternalServerError)
		return
	}
	stubs := make([]*api.JobListStub, 0, len(m.jobs))
	for _, job := range m.jobs {
		stubs = append(stubs, &api.JobListStub{
			ID:               *job.ID,
			Name:             *job.ID,
			Namespace:        *job.Namespace,
			Type:             derefString(job.Type),
			Stop:             derefBool(job.Stop),
			Periodic:         job.Periodic != nil,
			ParameterizedJob: job.ParameterizedJob != nil,
			ModifyIndex:      *job.ModifyIndex,
			JobModifyIndex:   *job.JobModifyIndex,
		})
	}
	sort.Slice(stubs, func(i, j int) bool { return stubs[i].ID < stubs[j].ID })
	w.Header().Set("X-Nomad-Index", strconv.FormatUint(m.index, 10))
	writeJSON(w, stubs)
}

func (m *mockNomad) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req api.JobRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Job == nil || req.Job.ID == nil {
		http.Error(w, "invalid job", http.StatusBadRequest)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeLocked(req.Job)
	m.registered = append(m.registered, req.Job)
	writeJSON(w, api.JobRegisterResponse{EvalID: "eval-register", JobModifyIndex: m.index})
}

func (m *mockNomad) handleJob(w http.ResponseWriter, r *http.Request, rest string) {
	action := ""
	for _, suffix := range []string{"/allocations", "/scale"} {
		if strings.HasSuffix(rest, suffix) {
			action = strings.TrimPrefix(suffix, "/")
			rest = strings.TrimSuffix(rest, suffix)
		}
	}
	jobID, err := url.PathUnescape(rest)
	if err != nil {
		http.Error(w, "bad job id", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[jobID]
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	switch action {
	case "":
		m.infoCalls[jobID]++
		writeJSON(w, job)
	case "allocations":
		allocs := m.allocs[jobID]
		if allocs == nil {
			allocs = []*api.AllocationListStub{}
		}
		writeJSON(w, allocs)
	case "scale":
		var req api.ScalingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Count == nil {
			http.Error(w, "invalid scaling request", http.StatusBadRequest)
			return
		}
		group := req.Target["Group"]
		var tg *api.TaskGroup
		for _, candidate := range job.TaskGroups {
			if *candidate.Name == group {
				tg = candidate
			}
		}
		if tg == nil {
			http.Error(w, "task group not found", http.StatusBadRequest)
			return
		}
		tg.Count = new(int(*req.Count))
		m.storeLocked(job)
		m.scaleCalls = append(m.scaleCalls, scaleCall{JobID: jobID, Group: group, Count: *req.Count, Message: req.Message})
		writeJSON(w, api.JobRegisterResponse{EvalID: "eval-scale", JobModifyIndex: m.index})
	}
}

func (m *mockNomad) handleStream(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	if m.streamFail {
		m.mu.Unlock()
		http.Error(w, "event stream unavailable", http.StatusInternalServerError)
		return
	}
	ch := make(chan []byte, 32)
	id := m.nextStream
	m.nextStream++
	m.streams[id] = ch
	m.mu.Unlock()
	m.streamOpens.Add(1)
	defer func() {
		m.mu.Lock()
		if current, ok := m.streams[id]; ok && current == ch {
			delete(m.streams, id)
		}
		m.mu.Unlock()
	}()

	flusher := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Nomad sends empty heartbeats to keep the connection alive.
	_, _ = w.Write([]byte("{}\n"))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	return b != nil && *b
}

// serviceJob builds a service job with one task group "web".
func serviceJob(id string, count int, meta map[string]string) *api.Job {
	return &api.Job{
		ID:        new(id),
		Name:      new(id),
		Namespace: new("default"),
		Type:      new(api.JobTypeService),
		Stop:      new(false),
		TaskGroups: []*api.TaskGroup{{
			Name:  new("web"),
			Count: new(count),
			Meta:  meta,
		}},
	}
}

var enabledMeta = map[string]string{"sablier.enable": "true"}

func runningAlloc(jobID string) *api.AllocationListStub {
	return &api.AllocationListStub{
		ID:               "alloc-" + jobID,
		JobID:            jobID,
		TaskGroup:        "web",
		ClientStatus:     api.AllocClientStatusRunning,
		DesiredStatus:    api.AllocDesiredStatusRun,
		DeploymentStatus: &api.AllocDeploymentStatus{Healthy: new(true)},
	}
}

// waitStreamOpens waits until the mock saw at least n event stream connections.
func waitStreamOpens(t *testing.T, m *mockNomad, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for m.streamOpens.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("event stream was opened %d times, want at least %d", m.streamOpens.Load(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
