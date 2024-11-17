package promise

import (
	"github.com/panjf2000/ants/v2"
	conc "github.com/sourcegraph/conc/pool"
)

var (
	defaultPool = newDefaultPool()
)

type Pool interface {
	Go(f func())
}

type wrapFunc func(f func())

func (wf wrapFunc) Go(f func()) {
	wf(f)
}

func newDefaultPool() Pool {
	return wrapFunc(func(f func()) {
		go f()
	})
}

func FromConcPool(p *conc.Pool) Pool {
	return wrapFunc(p.Go)
}

func FromAntsPool(p *ants.Pool) Pool {
	return wrapFunc(func(f func()) {
		p.Submit(f)
	})
}
