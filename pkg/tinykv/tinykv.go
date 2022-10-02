package tinykv

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type timeout struct {
	expiresAt    time.Time
	expiresAfter time.Duration
	isSliding    bool
	key          string
}

func newTimeout(
	key string,
	expiresAfter time.Duration,
	isSliding bool) *timeout {
	return &timeout{
		expiresAt:    time.Now().Add(expiresAfter),
		expiresAfter: expiresAfter,
		isSliding:    isSliding,
		key:          key,
	}
}

func (to *timeout) slide() {
	if to == nil {
		return
	}
	if !to.isSliding {
		return
	}
	if to.expiresAfter <= 0 {
		return
	}
	to.expiresAt = time.Now().Add(to.expiresAfter)
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
	Delete(k string)
	Get(k string) (v T, ok bool)
	Keys() (keys []string)
	Values() (values []T)
	Entries() (entries map[string]entry[T])
	Put(k string, v T, options ...PutOption) error
	Take(k string) (v T, ok bool)
	Stop()
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(b []byte) error
}

//-----------------------------------------------------------------------------

type putOpt struct {
	expiresAfter time.Duration
	isSliding    bool
	cas          func(interface{}, bool) bool
}

// PutOption extra options for put
type PutOption func(*putOpt)

// ExpiresAfter entry will expire after this time
func ExpiresAfter(expiresAfter time.Duration) PutOption {
	return func(opt *putOpt) {
		opt.expiresAfter = expiresAfter
	}
}

// IsSliding sets if the entry would get expired in a sliding manner
func IsSliding(isSliding bool) PutOption {
	return func(opt *putOpt) {
		opt.isSliding = isSliding
	}
}

// CAS for performing a compare and swap
func CAS(cas func(oldValue interface{}, found bool) bool) PutOption {
	return func(opt *putOpt) {
		opt.cas = cas
	}
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

// Delete deletes an entry
func (kv *store[T]) Delete(k string) {
	kv.mx.Lock()
	defer kv.mx.Unlock()
	delete(kv.kv, k)
}

// Get gets an entry from KV store
// and if a sliding timeout is set, it will be slided
func (kv *store[T]) Get(k string) (T, bool) {
	var zero T
	kv.mx.Lock()
	defer kv.mx.Unlock()

	e, ok := kv.kv[k]
	if !ok {
		return zero, ok
	}
	e.slide()
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
				isSliding:    v.isSliding,
				key:          k,
			}
			e.timeout = t
		}
		entries[k] = e
	}
	return entries
}

// Put puts an entry inside kv store with provided options
func (kv *store[T]) Put(k string, v T, options ...PutOption) error {
	opt := &putOpt{}
	for _, v := range options {
		v(opt)
	}
	e := &entry[T]{
		value: v,
	}
	kv.mx.Lock()
	defer kv.mx.Unlock()
	if opt.expiresAfter > 0 {
		e.timeout = newTimeout(k, opt.expiresAfter, opt.isSliding)
		timeheapPush(&kv.heap, e.timeout)
	}
	if opt.cas != nil {
		return kv.cas(k, e, opt.cas)
	}
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
			Value        T             `json:"value"`
			ExpiresAt    time.Time     `json:"expiresAt"`
			ExpiresAfter time.Duration `json:"expiresAfter"`
			IsSliding    bool          `json:"isSliding"`
		}{
			Value:        e.value,
			ExpiresAt:    e.expiresAt,
			ExpiresAfter: e.expiresAfter,
			IsSliding:    e.isSliding,
		})
	} else {
		return json.Marshal(&struct {
			Value T `json:"value"`
		}{
			Value: e.value,
		})
	}
}

type minimalEntry[T any] struct {
	Value        T
	ExpiresAfter time.Duration
}

func (kv *store[T]) UnmarshalJSON(b []byte) error {

	var result map[string]minimalEntry[T]

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal([]byte(b), &result)

	for k, v := range result {
		// TODO: Handle sliding...
		kv.Put(k, v.Value, ExpiresAfter(v.ExpiresAfter))
	}

	return nil
}

func (e *minimalEntry[T]) UnmarshalJSON(b []byte) error {

	result := &struct {
		Value     T         `json:"value"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{}

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal([]byte(b), &result)

	if result.ExpiresAt.After(time.Now()) {
		e.Value = result.Value
		e.ExpiresAfter = time.Until(result.ExpiresAt)
	}
	// TODO: Handle sliding...

	return nil
}

func (kv *store[T]) cas(k string, e *entry[T], casFunc func(interface{}, bool) bool) error {
	old, ok := kv.kv[k]
	var oldValue T
	if ok && old != nil {
		oldValue = old.value
	}
	if !casFunc(oldValue, ok) {
		return ErrCASCond
	}
	if ok && old != nil {
		if e.timeout != nil {
			old.timeout = e.timeout
		}
		old.value = e.value
		e = old
	}
	e.slide()
	kv.kv[k] = e
	return nil
}

// Take takes an entry out of kv store
func (kv *store[T]) Take(k string) (T, bool) {
	var zero T
	kv.mx.Lock()
	defer kv.mx.Unlock()
	e, ok := kv.kv[k]
	if ok {
		delete(kv.kv, k)
		return e.value, ok
	}
	return zero, ok
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

//-----------------------------------------------------------------------------

// errors
var (
	ErrCASCond = errorf("CAS COND FAILED")
)

//-----------------------------------------------------------------------------

type sentinelErr string

func (v sentinelErr) Error() string { return string(v) }
func errorf(format string, a ...interface{}) error {
	return sentinelErr(fmt.Sprintf(format, a...))
}

//-----------------------------------------------------------------------------
