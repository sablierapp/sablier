package nomad

import "sync"

// jobLocks hands out one lock per job ID. Nomad commits a scale request only if
// the job did not change since it was read, so two task groups of one job can't
// be scaled at the same time: the second request fails with a job modify index
// conflict. Holding the job's lock around each scale keeps them in line.
type jobLocks struct {
	mu    sync.Mutex
	locks map[string]*jobLock
}

type jobLock struct {
	mu   sync.Mutex
	refs int
}

// lock waits for the lock of jobID and returns the function that releases it.
// The entry is dropped once nobody holds or waits for it, so the map doesn't
// grow with every job ever scaled.
func (l *jobLocks) lock(jobID string) (unlock func()) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*jobLock)
	}
	jl, ok := l.locks[jobID]
	if !ok {
		jl = &jobLock{}
		l.locks[jobID] = jl
	}
	jl.refs++
	l.mu.Unlock()

	jl.mu.Lock()
	return func() {
		jl.mu.Unlock()
		l.mu.Lock()
		defer l.mu.Unlock()
		jl.refs--
		if jl.refs == 0 {
			delete(l.locks, jobID)
		}
	}
}

// lockJob locks the job of an instance name. A name that doesn't parse needs no
// lock: getGroup reports it as an error.
func (p *Provider) lockJob(name string) (unlock func()) {
	parsed, err := parseName(name)
	if err != nil {
		return func() {}
	}
	return p.scaleLocks.lock(parsed.JobID)
}
