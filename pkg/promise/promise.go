package promise

import (
	"context"
	"fmt"
	"sync"
)

type Status string

const (
	Pending   Status = "pending"
	Fulfilled Status = "fulfilled"
	Rejected  Status = "rejected"
)

// Promise represents the eventual completion (or failure) of an asynchronous operation and its resulting value
type Promise[T any] struct {
	status Status

	value *T
	err   error
	ch    chan struct{}
	once  sync.Once
}

func New[T any](
	executor func(resolve func(T), reject func(error)),
) *Promise[T] {
	return NewWithPool(executor, defaultPool)
}

func NewWithPool[T any](
	executor func(resolve func(T), reject func(error)),
	pool Pool,
) *Promise[T] {
	if executor == nil {
		panic("executor is nil")
	}
	if pool == nil {
		panic("pool is nil")
	}

	p := &Promise[T]{
		status: Pending,
		value:  nil,
		err:    nil,
		ch:     make(chan struct{}),
		once:   sync.Once{},
	}

	pool.Go(func() {
		defer p.handlePanic()
		executor(p.resolve, p.reject)
	})

	return p
}

func Then[A, B any](
	p *Promise[A],
	ctx context.Context,
	resolve func(A) (B, error),
) *Promise[B] {
	return ThenWithPool(p, ctx, resolve, defaultPool)
}

func ThenWithPool[A, B any](
	p *Promise[A],
	ctx context.Context,
	resolve func(A) (B, error),
	pool Pool,
) *Promise[B] {
	return NewWithPool(func(resolveB func(B), reject func(error)) {
		result, err := p.Await(ctx)
		if err != nil {
			reject(err)
			return
		}

		resultB, err := resolve(*result)
		if err != nil {
			reject(err)
			return
		}

		resolveB(resultB)
	}, pool)
}

func Catch[T any](
	p *Promise[T],
	ctx context.Context,
	reject func(err error) error,
) *Promise[T] {
	return CatchWithPool(p, ctx, reject, defaultPool)
}

func CatchWithPool[T any](
	p *Promise[T],
	ctx context.Context,
	reject func(err error) error,
	pool Pool,
) *Promise[T] {
	return NewWithPool(func(resolve func(T), internalReject func(error)) {
		result, err := p.Await(ctx)
		if err != nil {
			internalReject(reject(err))
		} else {
			resolve(*result)
		}
	}, pool)
}

func (p *Promise[T]) Status() Status {
	return p.status
}

func (p *Promise[T]) Fulfilled() bool {
	return p.status == Fulfilled
}

func (p *Promise[T]) Pending() bool {
	return p.status == Pending
}

func (p *Promise[T]) Rejected() bool {
	return p.status == Rejected
}

func (p *Promise[T]) Await(ctx context.Context) (*T, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.ch:
		return p.value, p.err
	}
}

func (p *Promise[T]) resolve(value T) {
	p.once.Do(func() {
		p.value = &value
		p.status = Fulfilled
		close(p.ch)
	})
}

func (p *Promise[T]) reject(err error) {
	p.once.Do(func() {
		p.err = err
		p.status = Rejected
		close(p.ch)
	})
}

func (p *Promise[T]) handlePanic() {
	err := recover()
	if err == nil {
		return
	}

	switch v := err.(type) {
	case error:
		p.reject(v)
	default:
		p.reject(fmt.Errorf("%+v", v))
	}
}

// AllSettled resolves when all promises have resolved or rejected
func AllSettled[T any](
	ctx context.Context,
	promises ...*Promise[T],
) *Promise[[]*Promise[T]] {
	return AllSettledWithPool(ctx, defaultPool, promises...)
}

func AllSettledWithPool[T any](
	ctx context.Context,
	pool Pool,
	promises ...*Promise[T],
) *Promise[[]*Promise[T]] {
	if len(promises) == 0 {
		panic("missing promises")
	}

	return NewWithPool[[]*Promise[T]](func(resolve func([]*Promise[T]), reject func(error)) {
		group := sync.WaitGroup{}
		group.Add(len(promises))

		for _, p := range promises {
			_ = ThenWithPool(p, ctx, func(data T) (T, error) {
				group.Done()
				return data, nil
			}, pool)
			_ = CatchWithPool(p, ctx, func(err error) error {
				group.Done()
				return err
			}, pool)
		}

		group.Wait()

		resolve(promises)
	}, pool)
}

// All resolves when all promises have resolved, or rejects immediately upon any of the promises rejecting
func All[T any](
	ctx context.Context,
	promises ...*Promise[T],
) *Promise[[]T] {
	return AllWithPool(ctx, defaultPool, promises...)
}

func AllWithPool[T any](
	ctx context.Context,
	pool Pool,
	promises ...*Promise[T],
) *Promise[[]T] {
	if len(promises) == 0 {
		panic("missing promises")
	}

	return NewWithPool(func(resolve func([]T), reject func(error)) {
		resultsChan := make(chan tuple[T, int], len(promises))
		errsChan := make(chan error, len(promises))

		for idx, p := range promises {
			idx := idx
			_ = ThenWithPool(p, ctx, func(data T) (T, error) {
				resultsChan <- tuple[T, int]{_1: data, _2: idx}
				return data, nil
			}, pool)
			_ = CatchWithPool(p, ctx, func(err error) error {
				errsChan <- err
				return err
			}, pool)
		}

		results := make([]T, len(promises))
		for idx := 0; idx < len(promises); idx++ {
			select {
			case result := <-resultsChan:
				results[result._2] = result._1
			case err := <-errsChan:
				reject(err)
				return
			}
		}
		resolve(results)
	}, pool)
}

// Race resolves or rejects as soon as any one of the promises resolves or rejects
func Race[T any](
	ctx context.Context,
	promises ...*Promise[T],
) *Promise[T] {
	return RaceWithPool(ctx, defaultPool, promises...)
}

func RaceWithPool[T any](
	ctx context.Context,
	pool Pool,
	promises ...*Promise[T],
) *Promise[T] {
	if len(promises) == 0 {
		panic("missing promises")
	}

	return NewWithPool(func(resolve func(T), reject func(error)) {
		valsChan := make(chan T, len(promises))
		errsChan := make(chan error, len(promises))

		for _, p := range promises {
			_ = ThenWithPool(p, ctx, func(data T) (T, error) {
				valsChan <- data
				return data, nil
			}, pool)
			_ = CatchWithPool(p, ctx, func(err error) error {
				errsChan <- err
				return err
			}, pool)
		}

		select {
		case val := <-valsChan:
			resolve(val)
		case err := <-errsChan:
			reject(err)
		}
	}, pool)
}

type tuple[T1, T2 any] struct {
	_1 T1
	_2 T2
}
