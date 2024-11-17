package promise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	conc "github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/require"
)

var (
	ctx         = context.Background()
	errExpected = errors.New("expected error")
)

func TestNew(t *testing.T) {
	p := New(func(resolve func(any), reject func(error)) {
		resolve(nil)
	})
	require.NotNil(t, p)
}

func TestNewWithPool(t *testing.T) {
	tests := []struct {
		name string
		pool Pool
	}{
		{
			name: "default",
			pool: newDefaultPool(),
		},
		{
			name: "conc",
			pool: func() Pool {
				return FromConcPool(conc.New())
			}(),
		},
		{
			name: "ants",
			pool: func() Pool {
				antsPool, err := ants.NewPool(0)
				require.NoError(t, err)
				return FromAntsPool(antsPool)
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := NewWithPool(func(resolve func(string), reject func(error)) {
				resolve(test.name)
			}, test.pool)

			val, err := p.Await(ctx)
			require.NoError(t, err)
			require.NotNil(t, val)
			require.Equal(t, test.name, *val)
		})
	}
}

func TestPromise_Then(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		resolve("Hello, ")
	})
	p2 := Then(p1, ctx, func(data string) (string, error) {
		return data + "world!", nil
	})
	p3 := Then(p2, ctx, func(_ string) (string, error) {
		return "", errExpected
	})

	val, err := p1.Await(ctx)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "Hello, ", *val)

	val, err = p2.Await(ctx)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "Hello, world!", *val)

	_, err = p3.Await(ctx)
	require.EqualError(t, err, errExpected.Error())
}

func TestPromise_Catch(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})

	val, err := p1.Await(ctx)
	require.Error(t, err)
	require.Equal(t, errExpected, err)
	require.Nil(t, val)
}

func TestPromise_Panic(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		panic("random error")
	})
	p2 := New(func(resolve func(any), reject func(error)) {
		panic(errExpected)
	})

	val, err := p1.Await(ctx)
	require.Error(t, err)
	require.Equal(t, errors.New("random error"), err)
	require.Nil(t, val)

	val, err = p2.Await(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errExpected)
	require.Nil(t, val)
}

func TestAllSettled_Happy(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		resolve("one")
	})
	p2 := New(func(resolve func(string), reject func(error)) {
		resolve("two")
	})
	p3 := New(func(resolve func(string), reject func(error)) {
		resolve("three")
	})

	p := AllSettled(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	promises := *val
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Len(t, *val, 3)
	require.Equal(t, "one", *(promises[0].value))
	require.Equal(t, "two", *(promises[1].value))
	require.Equal(t, "three", *(promises[2].value))
}

func TestAllSettled_ContainsRejected(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		resolve("one")
	})
	p2 := New(func(resolve func(string), reject func(error)) {
		reject(errExpected)
	})
	p3 := New(func(resolve func(string), reject func(error)) {
		resolve("three")
	})

	p := AllSettled(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	promises := *val
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Len(t, *val, 3)
	require.Equal(t, "one", *(promises[0].value))
	require.ErrorIs(t, promises[1].err, errExpected)
	require.Equal(t, "three", *(promises[2].value))
}

func TestAllSettled_OnlyRejected(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})
	p2 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})
	p3 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})

	p := AllSettled(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	promises := *val
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Len(t, *val, 3)
	require.ErrorIs(t, promises[0].err, errExpected)
	require.ErrorIs(t, promises[1].err, errExpected)
	require.ErrorIs(t, promises[2].err, errExpected)
}

func TestAll_Happy(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		resolve("one")
	})
	p2 := New(func(resolve func(string), reject func(error)) {
		resolve("two")
	})
	p3 := New(func(resolve func(string), reject func(error)) {
		resolve("three")
	})

	p := All(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, []string{"one", "two", "three"}, *val)
}

func TestAll_ContainsRejected(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		resolve("one")
	})
	p2 := New(func(resolve func(string), reject func(error)) {
		reject(errExpected)
	})
	p3 := New(func(resolve func(string), reject func(error)) {
		resolve("three")
	})

	p := All(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errExpected)
	require.Nil(t, val)
}

func TestAll_OnlyRejected(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})
	p2 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})
	p3 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})

	p := All(ctx, p1, p2, p3)

	val, err := p.Await(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errExpected)
	require.Nil(t, val)
}

func TestRace_Happy(t *testing.T) {
	p1 := New(func(resolve func(string), reject func(error)) {
		time.Sleep(time.Millisecond * 100)
		resolve("faster")
	})
	p2 := New(func(resolve func(string), reject func(error)) {
		time.Sleep(time.Millisecond * 500)
		resolve("slower")
	})

	p := Race(ctx, p1, p2)

	val, err := p.Await(ctx)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "faster", *val)
}

func TestRace_ContainsRejected(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		time.Sleep(time.Millisecond * 100)
		resolve(nil)
	})
	p2 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})

	p := Race(ctx, p1, p2)

	val, err := p.Await(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errExpected)
	require.Nil(t, val)
}

func TestRace_OnlyRejected(t *testing.T) {
	p1 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})
	p2 := New(func(resolve func(any), reject func(error)) {
		reject(errExpected)
	})

	p := Race(ctx, p1, p2)

	val, err := p.Await(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errExpected)
	require.Nil(t, val)
}
