package sablier_test

import (
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/provider/providertest"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/storetest"
	"go.uber.org/mock/gomock"
)

func setupSablier(t *testing.T) (*sablier.Sablier, *storetest.MockStore, *providertest.MockProvider) {
	t.Helper()
	ctrl := gomock.NewController(t)

	p := providertest.NewMockProvider(ctrl)
	s := storetest.NewMockStore(ctrl)

	m := sablier.New(slogt.New(t), s, p)
	return m, s, p
}

type fakeRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeRecorder) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}

func (f *fakeRecorder) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeRecorder) RecordSessionRequest(strategy, target, group string) {
	f.record("session:" + strategy + "/" + target + "/" + group)
}
func (f *fakeRecorder) RecordInstanceStartEnd(instance string, _ time.Duration) {
	f.record("start_end:" + instance)
}
func (f *fakeRecorder) RecordGroupStartDuration(string, time.Duration) {}
func (f *fakeRecorder) RecordInstanceStartFailure(instance string) {
	f.record("start_fail:" + instance)
}
func (f *fakeRecorder) RecordReadyWaitBegin(instance string)   { f.record("ready_begin:" + instance) }
func (f *fakeRecorder) RecordReadyWaitEnd(instance string)     { f.record("ready_end:" + instance) }
func (f *fakeRecorder) DiscardReadyWait(instance string)       { f.record("ready_discard:" + instance) }
func (f *fakeRecorder) RecordActiveInstance(instance string)   { f.record("active+:" + instance) }
func (f *fakeRecorder) RecordInactiveInstance(instance string) { f.record("active-:" + instance) }
func (f *fakeRecorder) RecordInstanceStop(instance, reason string) {
	f.record("stop:" + instance + "/" + reason)
}

// setupSablierWithMetrics is like setupSablier but installs a fakeRecorder.
func setupSablierWithMetrics(t *testing.T) (*sablier.Sablier, *storetest.MockStore, *providertest.MockProvider, *fakeRecorder) {
	t.Helper()
	m, s, p := setupSablier(t)
	r := &fakeRecorder{}
	var _ metrics.Recorder = r // compile-time interface check
	m.WithMetrics(r)
	return m, s, p, r
}
