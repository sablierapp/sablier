package tinykv

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeoutHeap(t *testing.T) {
	assert := assert.New(t)

	now := time.Now()
	r := rand.New(rand.NewSource(now.Unix()))
	n := r.Intn(10000) + 10
	var h th = []*timeout{}
	for i := 0; i < n; i++ {
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
	assert := assert.New(t)
	rg := New[int](0)
	defer rg.Stop()

	rg.Put("1", 1)
	v, ok := rg.Get("1")
	assert.True(ok)
	assert.Equal(1, v)

	rg.Put("2", 2, ExpiresAfter(time.Millisecond*50))
	v, ok = rg.Get("2")
	assert.True(ok)
	assert.Equal(2, v)
	<-time.After(time.Millisecond * 100)

	v, ok = rg.Get("2")
	assert.False(ok)
	assert.NotEqual(2, v)
}

func TestKeys(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0)
	defer rg.Stop()

	rg.Put("1", 1)
	rg.Put("2", 2)

	keys := rg.Keys()
	assert.NotEmpty(keys)
	assert.Contains(keys, "1")
	assert.Contains(keys, "2")
}

func TestValues(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0)
	defer rg.Stop()

	rg.Put("1", 1)
	rg.Put("2", 2)

	values := rg.Values()
	assert.NotEmpty(values)
	assert.Contains(values, 1)
	assert.Contains(values, 2)
}

func TestEntries(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0)
	defer rg.Stop()

	rg.Put("1", 1)
	rg.Put("2", 2)
	rg.Put("3", 3, ExpiresAfter(time.Minute*50))

	entries := rg.Entries()
	assert.NotEmpty(entries)
	assert.NotNil(entries["1"])
	assert.NotNil(entries["2"])
	assert.NotNil(entries["3"])
}

func TestMarshalJSON(t *testing.T) {
	assert := assert.New(t)
	rg := New[int](0)
	defer rg.Stop()

	rg.Put("1", 1)
	rg.Put("2", 2)
	rg.Put("3", 3, ExpiresAfter(time.Minute*50))

	jsonb, err := json.Marshal(rg)
	assert.Nil(err)
	json := string(jsonb)
	assert.Regexp(`{"1":{"value":1},"2":{"value":2},"3":{"value":3,"expiresAt":"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}.\d+Z"}}`, json)
}

func TestUnmarshalJSON(t *testing.T) {
	assert := assert.New(t)
	in5Minutes := time.Now().Add(time.Minute * 5)
	in5MinutesJson, err := json.Marshal(in5Minutes)
	assert.Nil(err)
	jsons := `{"1":{"value":1},"2":{"value":2},"3":{"value":3,"expiresAt":` + string(in5MinutesJson) + `}}`

	rg := New[int](0)
	defer rg.Stop()

	err = json.Unmarshal([]byte(jsons), &rg)
	assert.Nil(err)
}

func TestTimeout(t *testing.T) {
	assert := assert.New(t)
	rcvd := make(chan string, 100)
	notify := func(k string, v interface{}) {
		rcvd <- k
	}
	rg := New(time.Millisecond*10, notify)
	n := 1000
	for i := n; i < 2*n; i++ {
		rg.Put(strconv.Itoa(i), i, ExpiresAfter(time.Millisecond*10))
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
	for i := 0; i < n; i++ {
		if got[i] != "" {
			continue
		}
		assert.Fail("should have value", i, got[i])
	}
}

func Test03(t *testing.T) {
	assert := assert.New(t)
	var putAt time.Time
	elapsed := make(chan time.Duration, 1)
	kv := New(
		time.Millisecond*50,
		func(k string, v interface{}) {
			elapsed <- time.Since(putAt)
		})

	putAt = time.Now()
	kv.Put("1", 1, ExpiresAfter(time.Millisecond*10))

	<-time.After(time.Millisecond * 100)
	assert.WithinDuration(putAt, putAt.Add(<-elapsed), time.Millisecond*60)
}

func Test04(t *testing.T) {
	assert := assert.New(t)
	kv := New(
		time.Millisecond*10,
		func(k string, v interface{}) {
			t.Fatal(k, v)
		})

	err := kv.Put("1", 1, ExpiresAfter(time.Millisecond*10000))
	assert.NoError(err)
	<-time.After(time.Millisecond * 50)
	kv.Delete("1")
	kv.Delete("1")

	<-time.After(time.Millisecond * 100)
	_, ok := kv.Get("1")
	assert.False(ok)
}

func Test05(t *testing.T) {
	assert := assert.New(t)
	N := 10000
	var cnt int64
	kv := New(
		time.Millisecond*10,
		func(k string, v interface{}) {
			atomic.AddInt64(&cnt, 1)
		})

	src := rand.NewSource(time.Now().Unix())
	rnd := rand.New(src)
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("%d", i)
		kv.Put(k, fmt.Sprintf("VAL::%v", k),
			ExpiresAfter(
				time.Millisecond*time.Duration(rnd.Intn(10)+1)))
	}

	<-time.After(time.Millisecond * 100)
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("%d", i)
		_, ok := kv.Get(k)
		assert.False(ok)
	}
}

func Test07(t *testing.T) {
	assert := assert.New(t)

	kv := New[int](-1)
	kv.Put("1", 1)
	v, ok := kv.Take("1")
	assert.True(ok)
	assert.Equal(1, v)

	_, ok = kv.Get("1")
	assert.False(ok)
}

func Test08(t *testing.T) {
	assert := assert.New(t)

	kv := New[interface{}](-1)
	err := kv.Put(
		"QQG", "G",
		CAS(func(interface{}, bool) bool { return true }),
		ExpiresAfter(time.Millisecond))
	assert.NoError(err)

	v, ok := kv.Take("QQG")
	assert.True(ok)
	assert.Equal("G", v)
}

// ignore new timeouts when cas, and just use the old ones from the old value (if exists)
func Test09IgnoreTimeoutParamsOnCAS(t *testing.T) {
	assert := assert.New(t)

	key := "QQG"

	kv := New[interface{}](time.Millisecond)
	err := kv.Put(
		key, "G",
		CAS(func(interface{}, bool) bool { return true }),
		ExpiresAfter(time.Millisecond*30))
	assert.NoError(err)

	v, ok := kv.Get(key)
	assert.True(ok)
	assert.Equal("G", v)

	<-time.After(time.Millisecond * 20)

	err = kv.Put(key, "OK",
		CAS(func(currentValue interface{}, found bool) bool {
			assert.True(found)
			assert.Equal("G", currentValue)
			return true
		}))
	assert.NoError(err)

	<-time.After(time.Millisecond * 12)
	_, ok = kv.Get(key)
	assert.False(ok)
}

func Test11(t *testing.T) {
	assert := assert.New(t)

	key := "QQG"

	var expiredKey = make(chan string, 100)
	onExpired := func(k string, v interface{}) { expiredKey <- k }

	kv := New(time.Millisecond*100, onExpired)
	err := kv.Put(
		key, "G",
		ExpiresAfter(time.Millisecond*15))
	assert.NoError(err)

	<-time.After(time.Millisecond * 10)

	v, ok := kv.Get(key)
	assert.True(ok)
	assert.Equal("G", v)

	<-time.After(time.Millisecond * 10)

	_, ok = kv.Get(key)
	assert.False(ok)
	<-time.After(time.Millisecond)
	assert.Equal(key, <-expiredKey)

	<-time.After(time.Millisecond * 110)

	_, ok = kv.Get(key)
	assert.False(ok)
}

func Test12(t *testing.T) {
	assert := assert.New(t)

	key := "QQG"

	onExpired := func(k string, v interface{}) {}

	kv := New(time.Millisecond*100, onExpired)
	err := kv.Put(
		key, "G",
		ExpiresAfter(time.Millisecond))
	assert.NoError(err)

	<-time.After(time.Millisecond * 10)

	v, ok := kv.Get(key)
	assert.False(ok)
	assert.Equal(nil, v)
}

func Test13(t *testing.T) {
	assert := assert.New(t)

	got := make(chan interface{}, 10)
	onExpired := func(k string, v interface{}) {
		got <- v
	}

	kv := New(time.Millisecond*10, onExpired)
	err := kv.Put(
		"1", 123,
		ExpiresAfter(time.Millisecond))
	assert.NoError(err)

	<-time.After(time.Millisecond * 50)

	v, ok := kv.Get("1")
	assert.False(ok)
	assert.Equal(nil, v)

	v = <-got
	assert.Equal(123, v)
}

func TestOrdering(t *testing.T) {
	assert := assert.New(t)

	type data struct {
		key   string
		value interface{}
	}
	got := make(chan data, 100)
	onExpired := func(k string, v interface{}) {
		got <- data{k, v}
	}

	kv := New(time.Millisecond*5, onExpired)

	for i := 1; i <= 10; i++ {
		k := strconv.Itoa(i)
		v := i
		kv.Put(k, v, ExpiresAfter(time.Millisecond*time.Duration(i)*50))
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
}

func TestCASOldFound(t *testing.T) {
	assert := assert.New(t)

	kv := New[interface{}](time.Millisecond * 10)
	key := "KEY01"
	value := "VALUE01"
	err := kv.Put(
		key, value,
		CAS(func(old interface{}, found bool) bool {
			assert.Nil(old)
			assert.False(found)
			return true
		}))
	assert.NoError(err)
	err = kv.Put(
		key, value,
		CAS(func(old interface{}, found bool) bool {
			assert.Equal(value, old)
			assert.True(found)
			return true
		}))
	assert.NoError(err)
	kv.Delete(key)
	err = kv.Put(
		key, value,
		CAS(func(old interface{}, found bool) bool {
			assert.Nil(old)
			assert.False(found)
			return true
		}))
	assert.NoError(err)
	v, ok := kv.Take(key)
	assert.True(ok)
	assert.Equal(value, v)
	err = kv.Put(
		key, value,
		CAS(func(old interface{}, found bool) bool {
			assert.Nil(old)
			assert.False(found)
			return true
		}))
	assert.NoError(err)
}

func ExampleNew() {
	key := "KEY"
	value := "VALUE"

	kv := New[interface{}](time.Millisecond * 10)
	defer kv.Stop()
	kv.Put(key, value)
	v, ok := kv.Get(key)
	if !ok {
		// ...
	}
	fmt.Println(key, v)
	kv.Delete(key)
	_, ok = kv.Get(key)
	fmt.Println(ok)

	// Output:
	// KEY VALUE
	// false
}

func BenchmarkGetNoValue(b *testing.B) {
	rg := New[interface{}](-1)
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkGetValue(b *testing.B) {
	rg := New[interface{}](-1)
	rg.Put("1", 1)
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkGetSlidingTimeout(b *testing.B) {
	rg := New[interface{}](-1)
	rg.Put("1", 1, ExpiresAfter(time.Second*10))
	for n := 0; n < b.N; n++ {
		rg.Get("1")
	}
}

func BenchmarkPutOne(b *testing.B) {
	rg := New[interface{}](-1)
	for n := 0; n < b.N; n++ {
		rg.Put("1", 1)
	}
}

func BenchmarkPutN(b *testing.B) {
	rg := New[interface{}](-1)
	for n := 0; n < b.N; n++ {
		k := strconv.Itoa(n)
		rg.Put(k, n)
	}
}

func BenchmarkPutExpire(b *testing.B) {
	rg := New[interface{}](-1)
	for n := 0; n < b.N; n++ {
		rg.Put("1", 1, ExpiresAfter(time.Second*10))
	}
}

func BenchmarkCASTrue(b *testing.B) {
	rg := New[interface{}](-1)
	rg.Put("1", 1)
	for n := 0; n < b.N; n++ {
		rg.Put("1", 2, CAS(func(interface{}, bool) bool { return true }))
	}
}

func BenchmarkCASFalse(b *testing.B) {
	rg := New[interface{}](-1)
	rg.Put("1", 1)
	for n := 0; n < b.N; n++ {
		rg.Put("1", 2, CAS(func(interface{}, bool) bool { return false }))
	}
}
