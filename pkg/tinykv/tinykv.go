package tinykv

import (
	"encoding/json"
	"sync"
	"time"
)

type timeout struct {
	expiresAt    time.Time
	expiresAfter time.Duration
	key          string
}

func newTimeout(
	key string,
	expiresAfter time.Duration) *timeout {
	return &timeout{
		expiresAt:    time.Now().Add(expiresAfter),
		expiresAfter: expiresAfter,
		key:          key,
	}
}

func (to *timeout) expired() bool {
	if to == nil {
		return false
	}
	return time.Now().After(to.expiresAt)
}

//-----------------------------------------------------------------------------

// timeout heap
type th []*timeout

func (h th) Len() int           { return len(h) }
func (h th) Less(i, j int) bool { return h[i].expiresAt.Before(h[j].expiresAt) }
func (h th) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *th) Push(x tohVal)     { *h = append(*h, x) }
func (h *th) Pop() tohVal {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

//-----------------------------------------------------------------------------

type entry[T any] struct {
	*timeout
	value T
}

//-----------------------------------------------------------------------------

// KV is a registry for values (like/is a concurrent map) with timeout and sliding timeout
type KV[T any] interface {
	Get(k string) (v T, ok bool)
	Keys() (keys []string)
	Values() (values []T)
	Entries() (entries map[string]entry[T])
	Put(k string, v T, expiresAfter time.Duration) error
	Stop()
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(b []byte) error
}

//-----------------------------------------------------------------------------

// store is a registry for values (like/is a concurrent map) with timeout and sliding timeout
type store[T any] struct {
	onExpire func(k string, v T)

	stop               chan struct{}
	stopOnce           sync.Once
	expirationInterval time.Duration
	mx                 sync.Mutex
	kv                 map[string]*entry[T]
	heap               th
}

// New creates a new *store, onExpire is for notification (must be fast).
func New[T any](expirationInterval time.Duration, onExpire ...func(k string, v T)) KV[T] {
	if expirationInterval <= 0 {
		expirationInterval = time.Second * 20
	}
	res := &store[T]{
		stop:               make(chan struct{}),
		kv:                 make(map[string]*entry[T]),
		expirationInterval: expirationInterval,
		heap:               th{},
	}
	if len(onExpire) > 0 && onExpire[0] != nil {
		res.onExpire = onExpire[0]
	}
	go res.expireLoop()
	return res
}

// Stop stops the goroutine
func (kv *store[T]) Stop() {
	kv.stopOnce.Do(func() { close(kv.stop) })
}

func (kv *store[T]) Get(k string) (T, bool) {
	var zero T
	kv.mx.Lock()
	defer kv.mx.Unlock()

	e, ok := kv.kv[k]
	if !ok {
		return zero, ok
	}
	if e.expired() {
		go notifyExpirations(map[string]T{k: e.value}, kv.onExpire)
		delete(kv.kv, k)
		return zero, false
	}
	return e.value, ok
}

func (kv *store[T]) Keys() (keys []string) {
	kv.mx.Lock()
	defer kv.mx.Unlock()

	for k := range kv.kv {
		keys = append(keys, k)
	}
	return keys
}

func (kv *store[T]) Values() (values []T) {
	kv.mx.Lock()
	defer kv.mx.Unlock()

	for _, v := range kv.kv {
		values = append(values, v.value)
	}
	return values
}

func (kv *store[T]) Entries() (entries map[string]entry[T]) {
	kv.mx.Lock()
	defer kv.mx.Unlock()

	entries = make(map[string]entry[T])
	for k, v := range kv.kv {

		e := entry[T]{
			value: v.value,
		}
		if v.timeout != nil {

			t := &timeout{
				expiresAt:    v.expiresAt,
				expiresAfter: v.expiresAfter,
				key:          k,
			}
			e.timeout = t
		}
		entries[k] = e
	}
	return entries
}

// Put puts an entry inside kv store with provided options
func (kv *store[T]) Put(k string, v T, expiresAfter time.Duration) error {
	e := &entry[T]{
		value: v,
	}
	kv.mx.Lock()
	defer kv.mx.Unlock()

	e.timeout = newTimeout(k, expiresAfter)
	timeheapPush(&kv.heap, e.timeout)

	kv.kv[k] = e
	return nil
}

func (kv *store[T]) MarshalJSON() ([]byte, error) {
	kv.mx.Lock()
	defer kv.mx.Unlock()
	return json.Marshal(kv.kv)
}

func (e *entry[T]) MarshalJSON() ([]byte, error) {
	if e.timeout != nil {
		return json.Marshal(&struct {
			Value     T         `json:"value"`
			ExpiresAt time.Time `json:"expiresAt"`
		}{
			Value:     e.value,
			ExpiresAt: e.expiresAt,
		})
	}
	return nil, nil
}

type minimalEntry[T any] struct {
	Value        T
	ExpiresAfter time.Duration
	expired      bool
}

func (kv *store[T]) UnmarshalJSON(b []byte) error {

	var entries map[string]minimalEntry[T]

	if err := json.Unmarshal([]byte(b), &entries); err != nil {
		return err
	}

	for k, v := range entries {
		if !v.expired {
			kv.Put(k, v.Value, v.ExpiresAfter)
		}
	}

	return nil
}

func (e *minimalEntry[T]) UnmarshalJSON(b []byte) error {

	entry := &struct {
		Value     T         `json:"value"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{}

	if err := json.Unmarshal([]byte(b), &entry); err != nil {
		return err
	}

	if entry.ExpiresAt.After(time.Now()) {
		e.Value = entry.Value
		e.ExpiresAfter = time.Until(entry.ExpiresAt)
		e.expired = false
	} else {
		e.expired = true
	}
	return nil
}

//-----------------------------------------------------------------------------

func (kv *store[T]) expireLoop() {
	interval := kv.expirationInterval
	expireTime := time.NewTimer(interval)
	for {
		select {
		case <-kv.stop:
			return
		case <-expireTime.C:
			v := kv.expireFunc()
			if v < 0 {
				v = -1 * v
			}
			if v > 0 && v <= kv.expirationInterval {
				interval = (2*interval + v) / 3 // good enough history
			}
			if interval <= 0 {
				interval = time.Millisecond
			}
			expireTime.Reset(interval)
		}
	}
}

func (kv *store[T]) expireFunc() time.Duration {
	kv.mx.Lock()
	defer kv.mx.Unlock()

	var interval time.Duration
	if len(kv.heap) == 0 {
		return interval
	}
	expired := make(map[string]T)
	c := -1
	for {
		if len(kv.heap) == 0 {
			break
		}
		c++
		if c >= len(kv.heap) {
			break
		}
		last := kv.heap[0]
		entry, ok := kv.kv[last.key]
		if !ok {
			timeheapPop(&kv.heap)
			continue
		}
		if !last.expired() {
			interval = time.Until(last.expiresAt)
			if interval < 0 {
				interval = last.expiresAfter
			}
			break
		}
		last = timeheapPop(&kv.heap)
		if ok {
			expired[last.key] = entry.value
		}
	}
REVAL:
	for k := range expired {
		newVal, ok := kv.kv[k]
		if !ok ||
			newVal.timeout == nil ||
			!newVal.expired() {
			delete(expired, k)
			goto REVAL
		}
		delete(kv.kv, k)
	}
	go notifyExpirations(expired, kv.onExpire)
	if interval == 0 && len(kv.heap) > 0 {
		last := kv.heap[0]
		interval = time.Until(last.expiresAt)
		if interval < 0 {
			interval = last.expiresAfter
		}
	}
	return interval
}

func notifyExpirations[T any](
	expired map[string]T,
	onExpire func(k string, v T)) {
	if onExpire == nil {
		return
	}
	for k, v := range expired {
		k, v := k, v
		try(func() error {
			onExpire(k, v)
			return nil
		})
	}
}
