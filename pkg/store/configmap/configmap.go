// Package configmap persists the in-memory session store in a Kubernetes
// ConfigMap.
package configmap

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// DataKey is the binaryData key holding the gzipped JSON snapshot, in the
	// same format as storage.file.
	DataKey = "sessions.json.gz"

	// RefreshInterval bounds how often session refreshes are written. A
	// refresh only moves an expiry forward, so losing one on a crash ends
	// that session at most this much early.
	RefreshInterval = 30 * time.Second

	writeTimeout = 10 * time.Second
)

var _ sablier.Store = (*Store)(nil)

// Store is an in-memory store that persists its sessions in a ConfigMap.
//
// New and removed sessions are written immediately, refreshes of existing
// sessions at most every RefreshInterval. A single writer does every write, so
// a burst of new sessions results in at most one write in flight and one
// pending, each carrying the whole store at the time it starts.
type Store struct {
	*inmemory.InMemory

	cli  typedcorev1.ConfigMapInterface
	name string
	l    *slog.Logger

	// pending is set when the store changed since the last write, urgent
	// asks the writer to write now instead of on the next tick.
	pending atomic.Bool
	urgent  chan struct{}

	// mu serializes writes and guards cm, the ConfigMap as last read or
	// written. Writes update it, so labels or annotations set by others are
	// kept, and its resourceVersion detects concurrent changes.
	mu sync.Mutex
	cm *corev1.ConfigMap

	refreshInterval time.Duration
}

// New returns an empty store persisting into the ConfigMap name, accessed
// through cli. Call Load before serving, Run to start writing, and Save on
// shutdown.
func New(cli typedcorev1.ConfigMapInterface, name string, logger *slog.Logger) *Store {
	return &Store{
		InMemory:        inmemory.NewInMemory().(*inmemory.InMemory),
		cli:             cli,
		name:            name,
		l:               logger.With(slog.String("configmap", name)),
		urgent:          make(chan struct{}, 1),
		refreshInterval: RefreshInterval,
	}
}

// Put stores a session. A new session is written immediately, a refresh of an
// existing one on the next tick.
func (s *Store) Put(ctx context.Context, info sablier.InstanceInfo, duration time.Duration) error {
	_, getErr := s.Get(ctx, info.Name)
	if err := s.InMemory.Put(ctx, info, duration); err != nil {
		return err
	}
	s.changed(getErr != nil)
	return nil
}

// Delete removes a session and writes the store immediately.
func (s *Store) Delete(ctx context.Context, name string) error {
	_, getErr := s.Get(ctx, name)
	if err := s.InMemory.Delete(ctx, name); err != nil {
		return err
	}
	if getErr == nil {
		s.changed(true)
	}
	return nil
}

func (s *Store) changed(urgent bool) {
	s.pending.Store(true)
	if urgent {
		select {
		case s.urgent <- struct{}{}:
		default: // a write is already queued, and will carry this change
		}
	}
}

// Load restores the sessions persisted in the ConfigMap. A missing ConfigMap
// is not an error: it is created on the first write. An unreadable snapshot is
// logged and skipped, and overwritten by the next write. Any other error is
// returned: starting empty would overwrite the persisted sessions.
func (s *Store) Load(ctx context.Context) error {
	cm, err := s.cli.Get(ctx, s.name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		s.l.InfoContext(ctx, "no persisted sessions, the ConfigMap will be created on the first write")
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read ConfigMap %s: %w", s.name, err)
	}

	s.mu.Lock()
	s.cm = cm
	s.mu.Unlock()

	data, ok := cm.BinaryData[DataKey]
	if !ok {
		return nil
	}
	if err := s.restore(data); err != nil {
		s.l.WarnContext(ctx, "cannot restore persisted sessions, starting fresh", slog.Any("error", err))
		return nil
	}
	restored := 0
	_ = s.Range(ctx, func(sablier.InstanceInfo, time.Time) { restored++ })
	s.l.InfoContext(ctx, "sessions restored", slog.Int("sessions", restored))
	return nil
}

func (s *Store) restore(data []byte) error {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return s.UnmarshalJSON(raw)
}

// Run writes the store whenever it changed, until ctx is cancelled.
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.urgent:
		case <-ticker.C:
		}
		if err := s.Save(ctx); err != nil {
			s.l.ErrorContext(ctx, "cannot persist sessions", slog.Any("error", err))
		}
	}
}

// Save writes the store if it changed since the last write. A failed write is
// retried on the next tick.
func (s *Store) Save(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear before the snapshot: a change made while writing is written next.
	if !s.pending.Swap(false) {
		return nil
	}
	// This write carries any change a queued signal was asking for.
	select {
	case <-s.urgent:
	default:
	}
	data, err := s.snapshot()
	if err != nil {
		return err
	}
	// The API server rejects a ConfigMap over this size. Retrying would not
	// help until sessions end, so the change is dropped instead of retried.
	if len(data) > corev1.MaxSecretSize {
		return fmt.Errorf("snapshot is %d bytes, over the ConfigMap limit of %d bytes: not persisted", len(data), corev1.MaxSecretSize)
	}

	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := s.write(ctx, data); err != nil {
		s.pending.Store(true)
		return err
	}
	return nil
}

func (s *Store) snapshot() ([]byte, error) {
	raw, err := s.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("cannot marshal sessions: %w", err)
	}
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// write stores data in the ConfigMap, creating it when needed. Sablier is the
// only writer, so a conflict means someone else edited the ConfigMap: it is
// read again and overwritten.
func (s *Store) write(ctx context.Context, data []byte) error {
	if s.cm == nil {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:   s.name,
				Labels: map[string]string{"app.kubernetes.io/managed-by": "sablier"},
			},
			BinaryData: map[string][]byte{DataKey: data},
		}
		created, err := s.cli.Create(ctx, cm, metav1.CreateOptions{})
		if err == nil {
			s.cm = created
			return nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("cannot create ConfigMap %s: %w", s.name, err)
		}
	} else {
		err := s.update(ctx, s.cm, data)
		if err == nil || !apierrors.IsConflict(err) {
			return err
		}
	}

	s.l.WarnContext(ctx, "the ConfigMap was changed by someone else, overwriting it")
	current, err := s.cli.Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot read ConfigMap %s: %w", s.name, err)
	}
	return s.update(ctx, current, data)
}

func (s *Store) update(ctx context.Context, cm *corev1.ConfigMap, data []byte) error {
	cm = cm.DeepCopy()
	if cm.BinaryData == nil {
		cm.BinaryData = map[string][]byte{}
	}
	cm.BinaryData[DataKey] = data
	updated, err := s.cli.Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsConflict(err) {
			return err
		}
		return fmt.Errorf("cannot update ConfigMap %s: %w", s.name, err)
	}
	s.cm = updated
	return nil
}
