package tinykv

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutHeap(t *testing.T) {
	assert := assert.New(t)

	now := time.Now()
	r := rand.New(rand.NewSource(now.Unix()))
	n := r.Intn(10000) + 10
	var h th = []*timeout{}
	for range n {
		to := &timeout{expiresAt: now.Add(time.Duration(r.Intn(100000)) * time.Second)}
		timeheapPush(&h, to)
	}

	var prev *timeout
	// t.Log(h[0].expiresAt, h[len(h)-1].expiresAt)
	for len(h) > 0 {
		ito := timeheapPop(&h)
		if prev != nil {
			assert.Condition(func() bool { return !prev.expiresAt.After(ito.expiresAt) })
		}
		prev = ito
	}
	assert.Equal(0, len(h))
}

var _ KV[int] = &store[int]{}

func TestGetPut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		rg := New[int](0, nil)
		defer rg.Stop()

		require.NoError(t, rg.Put("1", 1, time.Minute*50))
		v, ok := rg.Get("1")
		assert.True(ok)
		assert.Equal(1, v)

		require.NoError(t, rg.Put("2", 2, time.Millisecond*50))
		v, ok = rg.Get("2")
		assert.True(ok)
		assert.Equal(2, v)
		synctest.Sleep(time.Millisecond * 100)

		v, ok = rg.Get("2")
		assert.False(ok)
		assert.NotEqual(2, v)
	})
}

func TestKeys(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0, nil)
	defer rg.Stop()

	require.NoError(t, rg.Put("1", 1, time.Minute*50))
	require.NoError(t, rg.Put("2", 2, time.Minute*50))

	keys := rg.Keys()
	assert.NotEmpty(keys)
	assert.Contains(keys, "1")
	assert.Contains(keys, "2")
}

func TestValues(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0, nil)
	defer rg.Stop()

	assert.NoError(rg.Put("1", 1, time.Minute*50))
	assert.NoError(rg.Put("2", 2, time.Minute*50))

	values := rg.Values()
	assert.NotEmpty(values)
	assert.Contains(values, 1)
	assert.Contains(values, 2)
}

func TestEntries(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0, nil)
	defer rg.Stop()

	assert.NoError(rg.Put("1", 1, time.Minute*50))
	assert.NoError(rg.Put("2", 2, time.Minute*50))
	assert.NoError(rg.Put("3", 3, time.Minute*50))

	entries := rg.Entries()
	assert.NotEmpty(entries)
	assert.NotNil(entries["1"])
	assert.NotNil(entries["2"])
	assert.NotNil(entries["3"])
}

func TestRange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		rg := New[int](0, nil)
		defer rg.Stop()

		require.NoError(t, rg.Put("live1", 1, time.Minute*50))
		require.NoError(t, rg.Put("live2", 2, time.Minute*50))
		require.NoError(t, rg.Put("expired", 3, time.Millisecond))

		synctest.Sleep(time.Millisecond * 20)

		got := make(map[string]int)
		expiries := make(map[string]time.Time)
		rg.Range(func(key string, value int, expiresAt time.Time) {
			got[key] = value
			expiries[key] = expiresAt
		})

		// Expired entries must never be yielded.
		assert.Len(got, 2)
		assert.Equal(1, got["live1"])
		assert.Equal(2, got["live2"])
		_, ok := got["expired"]
		assert.False(ok, "expired entries must not be yielded by Range")

		// The reported expiry is in the future for live entries.
		assert.True(expiries["live1"].After(time.Now()))

		// Range must not renew timeouts: the reported expiry is stable across calls.
		first := expiries["live1"]
		synctest.Sleep(time.Millisecond * 20)
		var second time.Time
		rg.Range(func(key string, _ int, expiresAt time.Time) {
			if key == "live1" {
				second = expiresAt
			}
		})
		assert.Equal(first, second, "Range must not renew an entry's timeout")
	})
}

func TestMarshalJSON(t *testing.T) {
	require.NoError(t, os.Setenv("TZ", ""))
	assert := assert.New(t)
	rg := New[int](0, nil)
	defer rg.Stop()

	assert.NoError(rg.Put("3", 3, time.Minute*50))

	jsonb, err := json.Marshal(rg)
	assert.Nil(err)
	json := string(jsonb)
	assert.Regexp(`{"3":{"value":3,"expiresAt":"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[\.\d]*(Z|[+-]\d{2}:\d{2})"}}`, json)
}

func TestUnmarshalJSON(t *testing.T) {
	assert := assert.New(t)
	in5Minutes := time.Now().Add(time.Minute * 5)
	in5MinutesJson, err := json.Marshal(in5Minutes)
	assert.Nil(err)
	jsons := `{"1":{"value":1},"2":{"value":2},"3":{"value":3,"expiresAt":` + string(in5MinutesJson) + `}}`

	rg := New[int](0, nil)
	defer rg.Stop()

	err = json.Unmarshal([]byte(jsons), &rg)
	assert.Nil(err)

	assert.Len(rg.Entries(), 1)
}

func TestUnmarshalJSONExpired(t *testing.T) {
	assert := assert.New(t)
	since5Minutes := time.Now().Add(-time.Minute * 5)
	since5MinutesJson, err := json.Marshal(since5Minutes)
	assert.Nil(err)
	jsons := `{"1":{"value":1},"2":{"value":2},"3":{"value":3,"expiresAt":` + string(since5MinutesJson) + `}}`

	rg := New[int](0, nil)
	defer rg.Stop()

	err = json.Unmarshal([]byte(jsons), &rg)
	assert.Nil(err)

	assert.Empty(rg.Entries())
}

func TestTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		rcvd := make(chan string, 100)
		notify := func(k string, v any) {
			rcvd <- k
		}
		rg := New(time.Millisecond*10, notify)
		defer rg.Stop()
		n := 1000
		for i := n; i < 2*n; i++ {
			assert.NoError(rg.Put(strconv.Itoa(i), i, time.Millisecond*10))
		}
		got := make([]string, n)
	OUT01:
		for {
			select {
			case v := <-rcvd:
				i, err := strconv.Atoi(v)
				assert.NoError(err)
				i = i - n
				if i < 0 || i >= n {
					t.Fail()
				}
				got[i] = v
			case <-time.After(time.Millisecond * 100):
				break OUT01
			}
		}
		assert.Equal(len(got), n)
		for i := range n {
			if got[i] != "" {
				continue
			}
			assert.Fail("should have value", i, got[i])
		}
	})
}

func Test03(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		var putAt time.Time
		elapsed := make(chan time.Duration, 1)
		kv := New(
			time.Millisecond*50,
			func(k string, v any) {
				elapsed <- time.Since(putAt)
			})
		defer kv.Stop()

		putAt = time.Now()
		require.NoError(t, kv.Put("1", 1, time.Millisecond*10))

		synctest.Sleep(time.Millisecond * 100)
		assert.WithinDuration(putAt, putAt.Add(<-elapsed), time.Millisecond*60)
	})
}

func Test04(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		kv := New(
			time.Millisecond*10,
			func(k string, v any) {
				t.Fatal(k, v)
			})
		defer kv.Stop()

		err := kv.Put("1", 1, time.Millisecond*10000)
		assert.NoError(err)
		synctest.Sleep(time.Millisecond * 50)
		kv.Delete("1")
		kv.Delete("1")

		synctest.Sleep(time.Millisecond * 100)
		_, ok := kv.Get("1")
		assert.False(ok)
	})
}

func Test05(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)
		N := 10000
		var cnt atomic.Int64
		kv := New(
			time.Millisecond*10,
			func(k string, v any) {
				cnt.Add(1)
			})
		defer kv.Stop()

		src := rand.NewSource(time.Now().Unix())
		rnd := rand.New(src)
		for i := range N {
			k := fmt.Sprintf("%d", i)
			require.NoError(t, kv.Put(k, fmt.Sprintf("VAL::%v", k),
				time.Millisecond*time.Duration(rnd.Intn(10)+1)))
		}

		synctest.Sleep(time.Millisecond * 100)
		for i := range N {
			k := fmt.Sprintf("%d", i)
			_, ok := kv.Get(k)
			assert.False(ok)
		}
	})
}

func Test11(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)

		key := "QQG"

		var expiredKey = make(chan string, 100)
		onExpired := func(k string, v any) { expiredKey <- k }

		kv := New(time.Millisecond*100, onExpired)
		defer kv.Stop()
		err := kv.Put(
			key, "G",
			time.Millisecond*15)
		assert.NoError(err)

		synctest.Sleep(time.Millisecond * 10)

		v, ok := kv.Get(key)
		assert.True(ok)
		assert.Equal("G", v)

		synctest.Sleep(time.Millisecond * 10)

		_, ok = kv.Get(key)
		assert.False(ok)
		synctest.Wait()
		assert.Equal(key, <-expiredKey)

		synctest.Sleep(time.Millisecond * 110)

		_, ok = kv.Get(key)
		assert.False(ok)
	})
}

func Test12(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)

		key := "QQG"

		onExpired := func(k string, v any) {}

		kv := New(time.Millisecond*100, onExpired)
		defer kv.Stop()
		err := kv.Put(
			key, "G",
			time.Millisecond)
		assert.NoError(err)

		synctest.Sleep(time.Millisecond * 10)

		v, ok := kv.Get(key)
		assert.False(ok)
		assert.Equal(nil, v)
	})
}

func Test13(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)

		got := make(chan any, 10)
		onExpired := func(k string, v any) {
			got <- v
		}

		kv := New(time.Millisecond*10, onExpired)
		defer kv.Stop()
		err := kv.Put(
			"1", 123,
			time.Millisecond)
		assert.NoError(err)

		synctest.Sleep(time.Millisecond * 50)

		v, ok := kv.Get("1")
		assert.False(ok)
		assert.Equal(nil, v)

		v = <-got
		assert.Equal(123, v)
	})
}

func TestOrdering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		assert := assert.New(t)

		type data struct {
			key   string
			value any
		}
		got := make(chan data, 100)
		onExpired := func(k string, v any) {
			got <- data{k, v}
		}

		kv := New(time.Millisecond*5, onExpired)
		defer kv.Stop()

		for i := 1; i <= 10; i++ {
			k := strconv.Itoa(i)
			v := i
			assert.NoError(kv.Put(k, v, time.Millisecond*time.Duration(i)*50))
		}

		var order = make([]int, 10)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case v := <-got:
					i, _ := strconv.Atoi(v.key)
					i--
					val := v.value.(int)
					val--
					order[i] = val
				case <-time.After(time.Millisecond * 100):
					return
				}
			}
		}()
		<-done
		for k, v := range order {
			assert.Equal(k, v)
		}

		assert.Equal(1, 1)
	})
}

// putAt adds an entry with a given expiry time. It keeps the timers from
// earlier calls in the heap, the same as Put.
func putAt[T any](kv *store[T], k string, v T, expiresAt time.Time) {
	to := &timeout{
		expiresAt:    expiresAt,
		expiresAfter: time.Until(expiresAt),
		key:          k,
	}
	kv.kv[k] = &entry[T]{timeout: to, value: v}
	timeheapPush(&kv.heap, to)
}

// One expiry cycle can pop an expired key and a stale timer of a renewed key.
// See https://github.com/sablierapp/sablier/issues/1110
func TestExpireFuncNotifiesExpiredKeyWithRenewedNeighbour(t *testing.T) {
	for i := range 200 {
		expired := make(chan string, 4)
		kv := &store[int]{
			onExpire:           func(k string, _ int) { expired <- k },
			stop:               make(chan struct{}),
			kv:                 make(map[string]*entry[int]),
			expirationInterval: time.Hour,
			heap:               th{},
		}

		// Key "a" is expired. Key "b" has a stale timer and a new timer.
		past := time.Now().Add(-time.Minute)
		putAt(kv, "a", 1, past)
		timeheapPush(&kv.heap, &timeout{expiresAt: past, expiresAfter: time.Minute, key: "b"})
		putAt(kv, "b", 2, time.Now().Add(time.Hour))

		kv.expireFunc()

		select {
		case k := <-expired:
			require.Equalf(t, "a", k, "iteration %d, only the idle key must expire", i)
		case <-time.After(time.Second):
			t.Fatalf("iteration %d, onExpire did not fire for the expired key", i)
		}

		_, ok := kv.kv["a"]
		assert.Falsef(t, ok, "iteration %d, the store must not contain the expired key", i)
		_, ok = kv.kv["b"]
		assert.Truef(t, ok, "iteration %d, the store must contain the renewed key", i)
	}
}

// The same defect through the public API.
// See https://github.com/sablierapp/sablier/issues/1110
func TestRenewedKeyKeepsNeighbourNotification(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for i := range 5 {
			var mu sync.Mutex
			fired := map[string]int{}
			kv := New[int](20*time.Millisecond, func(k string, _ int) {
				mu.Lock()
				fired[k]++
				mu.Unlock()
			})

			require.NoError(t, kv.Put("a", 1, 100*time.Millisecond))
			require.NoError(t, kv.Put("b", 1, 100*time.Millisecond))
			synctest.Sleep(60 * time.Millisecond)
			require.NoError(t, kv.Put("b", 2, time.Hour))
			synctest.Sleep(300 * time.Millisecond)

			_, ok := kv.Get("a")
			assert.Falsef(t, ok, "iteration %d, the store must not contain the idle key", i)

			mu.Lock()
			assert.Positivef(t, fired["a"], "iteration %d, onExpire must fire for the idle key", i)
			assert.Zerof(t, fired["b"], "iteration %d, the renewed key must not expire", i)
			mu.Unlock()

			kv.Stop()
		}
	})
}

func BenchmarkGetNoValue(b *testing.B) {
	rg := New[any](-1, nil)
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkGetValue(b *testing.B) {
	rg := New[any](-1, nil)
	assert.NoError(b, rg.Put("1", 1, time.Minute*50))
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkGetSlidingTimeout(b *testing.B) {
	rg := New[any](-1, nil)
	assert.NoError(b, rg.Put("1", 1, time.Second*10))
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkPutExpire(b *testing.B) {
	rg := New[any](-1, nil)
	for n := 0; n < b.N; n++ {
		assert.NoError(b, rg.Put("1", 1, time.Second*10))
	}
}
