package metrics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/metrics"
)

func TestNoopRecorderImplementsAllMethods(t *testing.T) {
	var r metrics.Recorder = metrics.Noop{}

	r.RecordSessionRequest("dynamic", "names")
	r.RecordInstanceStartEnd("nginx", 250*time.Millisecond)
	r.RecordInstanceStartFailure("nginx")
	r.RecordReadyWaitBegin("nginx")
	r.RecordReadyWaitEnd("nginx")
	r.RecordActiveInstance("nginx")
	r.RecordInactiveInstance("nginx")
	r.RecordInstanceStop("nginx", "expired")
	r.RecordActivityBytes("nginx", 1024)
	r.SetActivityWatched(1)

	_ = errors.Is(nil, nil)
}
