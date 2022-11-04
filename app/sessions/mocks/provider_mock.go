package mocks

import (
	"context"
	"sync"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/providers"
	"github.com/acouvreur/sablier/pkg/tinykv"
	"github.com/stretchr/testify/mock"
)

type ProviderMock struct {
	stoppedInstances []string

	wg sync.WaitGroup

	providers.Provider
	mock.Mock
}

func NewProviderMock(stoppedInstances []string) *ProviderMock {
	return &ProviderMock{
		stoppedInstances: stoppedInstances,
	}
}

func (provider *ProviderMock) NotifyInsanceStopped(ctx context.Context, instance chan string) {
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

type KVMock[T any] struct {
	wg sync.WaitGroup

	tinykv.KV[T]
	mock.Mock
}

func NewKVMock() *KVMock[instance.State] {
	return &KVMock[instance.State]{}
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
