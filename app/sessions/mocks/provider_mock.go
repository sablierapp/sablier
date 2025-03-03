package mocks

import (
	"context"
	"github.com/sablierapp/sablier/pkg/provider"
	"sync"
	"time"

	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/pkg/tinykv"
	"github.com/stretchr/testify/mock"
)

type ProviderMock struct {
	stoppedInstances []string

	wg sync.WaitGroup

	provider.Provider
	mock.Mock
}

func NewProviderMock() *ProviderMock {
	return &ProviderMock{}
}

func NewProviderMockWithStoppedInstancesEvents(stoppedInstances []string) *ProviderMock {
	return &ProviderMock{
		stoppedInstances: stoppedInstances,
	}
}

func (provider *ProviderMock) NotifyInstanceStopped(ctx context.Context, instance chan<- string) {
	go func() {
		defer close(instance)
		for i := 0; i < len(provider.stoppedInstances); i++ {
			instance <- provider.stoppedInstances[i]
		}
		provider.wg.Done()
	}()
}

func (provider *ProviderMock) Add(count int) {
	provider.wg.Add(count)
}

func (provider *ProviderMock) Wait() {
	provider.wg.Wait()
}

func (provider *ProviderMock) InstanceInspect(ctx context.Context, name string) (instance.State, error) {
	args := provider.Mock.Called(name)
	return args.Get(0).(instance.State), args.Error(1)
}

func (provider *ProviderMock) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	return make(map[string][]string), nil
}

type KVMock[T any] struct {
	wg sync.WaitGroup

	tinykv.KV[T]
	mock.Mock
}

func NewKVMock() *KVMock[instance.State] {
	return &KVMock[instance.State]{}
}

func (kv *KVMock[T]) Get(k string) (v instance.State, ok bool) {
	args := kv.Mock.Called(k)
	return args.Get(0).(instance.State), args.Bool(1)
}

func (kv *KVMock[T]) Delete(k string) {
	kv.Mock.Called(k)
	kv.wg.Done()
}

func (kv *KVMock[T]) Add(count int) {
	kv.wg.Add(count)
}

func (kv *KVMock[T]) Wait() {
	kv.wg.Wait()
}

func (kv *KVMock[T]) Put(k string, v T, expiresAfter time.Duration) error {
	return nil
}
