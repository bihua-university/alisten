package service

import (
	"context"
	"sync"
)

type flightCall[V any] struct {
	wg    sync.WaitGroup
	value V
	err   error
}

type flightGroup[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*flightCall[V]
}

func (g *flightGroup[K, V]) do(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*flightCall[V])
	}
	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		call.wg.Wait()
		return call.value, call.err
	}
	call := new(flightCall[V])
	call.wg.Add(1)
	g.calls[key] = call
	g.mu.Unlock()

	call.value, call.err = fn(ctx)
	g.mu.Lock()
	delete(g.calls, key)
	call.wg.Done()
	g.mu.Unlock()
	return call.value, call.err
}
