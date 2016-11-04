// Package spara provides a couple of functions for concurrent mapping over elements of a slice with
// early cancellation.
package spara

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidWorkers    = errors.New("spara: invalid number of workers")
	ErrInvalidIterations = errors.New("spara: invalid number of iterations")
)

func Run(workers int, iterations int, fn func(int) error) error {
	wrapped := func(_ context.Context, idx int) error {
		return fn(idx)
	}
	return RunWithContext(workers, iterations, wrapped)
}

func RunWithContext(workers int, iterations int, fn func(context.Context, int) error) error {

	if workers <= 0 {
		return ErrInvalidWorkers
	}

	if iterations < 0 {
		return ErrInvalidIterations
	}

	if fn == nil {
		return nil
	}

	if iterations == 0 {
		return nil
	}

	if workers > iterations {
		workers = iterations
	}

	var index int32 = int32(workers - 1)
	nextIndex := func() int {
		return int(atomic.AddInt32(&index, 1))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var firsterr error
	var killed int64
	kill := func(err error) {
		if atomic.CompareAndSwapInt64(&killed, 0, 1) {
			cancel()
			firsterr = err
		}
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(start int) {
			defer wg.Done()
			for j := start; j < iterations; j = nextIndex() {
				if atomic.LoadInt64(&killed) == 1 {
					return
				}
				if err := fn(ctx, j); err != nil {
					kill(err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	return firsterr
}
