// Package spara provides a couple of functions for concurrent mapping over
// elements of a slice with early cancellation.
package spara

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidWorkers     = errors.New("spara: invalid number of workers")
	ErrInvalidIterations  = errors.New("spara: invalid number of iterations")
	ErrNilMappingFunction = errors.New("spara: mapping function must not be nil")
	ErrNilContext         = errors.New("spara: context must not be nil")
)

// Run takes a mapping function and then runs it concurrently across up to
// workers separate goroutines, calling it with every number in the range [0,
// iterations-1]. If the mapping function returns an error, iteration will
// stop prematurely and Run will return that error once all outstanding calls
// to the mapping function complete.
//
// In more human terms, Run is just an interesting way to write a concurrent
// version of the higher order function, map. Because go doesn't have
// generics, we can't really write map without a bunch of type assertions.
// But, if instead of passing each value of the array we passed each _index_,
// we can recover some of the convenience of map with just a little more
// boilerplate.
func Run(workers int, iterations int, fn func(index int) error) error {
	if fn == nil {
		return ErrNilMappingFunction
	}
	wrapped := func(_ context.Context, index int) error { return fn(index) }
	return RunWithContext(context.Background(), workers, iterations, wrapped)
}

type MappingFunc func(ctx context.Context, index int) error

// RunWithContext is exactly like Run except:
//
// RunWithContext accepts a parent context. Completion of this context is
// tracked via it's Done() channel and will stop iteration early if possible.
// If completion _does_ cause iteration to stop, the error returned from
// RunWithContext will be the value of parent.Err().
//
// RunWithContext passes a context to the mapping function as well. This will
// be a child of the provided parent context, and will complete either on the
// first returned error, the parent completing, or all of the work being done.
//
// This method can give very large performance improvements when elements of
// the mapping function support context for early cancellation (eg
// http.Request's WithContext). Imagine a function that downloads multiple
// files in parallel. Using Run, the first error would cause no _new_ files to
// begin downloading, but it still would not return until all of the in
// progress downloads completed. If the in progress downloads are large, that
// could mean you're waiting a very long time for a bunch of data you don't
// actually care about. With early cancellation, these requests would be
// canceled eagerly, and the function could return much more quickly.
func RunWithContext(parent context.Context, workers int, iterations int, fn MappingFunc) error {
	if workers <= 0 {
		return ErrInvalidWorkers
	}
	if iterations < 0 {
		return ErrInvalidIterations
	}
	if fn == nil {
		return ErrNilMappingFunction
	}
	if parent == nil {
		return ErrNilContext
	}
	if iterations == 0 {
		return nil
	}

	// Only need to spawn as many workers as we have iterations.
	if workers > iterations {
		workers = iterations
	}

	// Eagerly check whether the parent context is already done.
	select {
	case <-parent.Done():
		return parent.Err()
	default:
		break
	}

	// Create a function that atomically returns the next index to process. We
	// can start this at workers-1, since the workers are passed their first
	// index directly.
	var index int32 = int32(workers - 1)
	nextIndex := func() int {
		return int(atomic.AddInt32(&index, 1))
	}
	// Atomically stops iteration inside of the worker functions.
	stopIteration := func() {
		atomic.StoreInt32(&index, int32(iterations))
	}

	// Wrap the parent context with cancellation so that we can stop internal
	// processing whenever a worker returns an error.
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var killOnce int32
	var firsterr error
	kill := func(err error) {
		// Only execute this on the first call. Only worker functions call
		// kill, so we can be certain that firsterr is safe to access once
		// wg.Done unblocks.
		if atomic.CompareAndSwapInt32(&killOnce, 0, 1) {
			stopIteration()
			cancel()
			firsterr = err
		}
	}

	// Start a goroutine that will stop iteration on parent context done. Must
	// be specified _after_ we wrap the parent context WithCancel; if parent
	// never completed, the spawned goroutine would leak.
	//
	// We don't need to spawn if the parent context never completes. The
	// stdlib's context.Background's Done method returns nil, so apparently we
	// can check that to decide what to do.
	if parent.Done() != nil {
		go func() {
			<-ctx.Done()
			if atomic.CompareAndSwapInt32(&killOnce, 0, 2) {
				stopIteration()
			}
		}()
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(start int) {
			defer wg.Done()
			for j := start; j < iterations; j = nextIndex() {
				if err := fn(ctx, j); err != nil {
					kill(err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	// killOnce = 1
	if firsterr != nil {
		return firsterr
	}

	// firsterr is nil, but the parent context may have been the thing that
	// stopped iteration, ie killOnce = 2. We know it definitely hasn't if the
	// parent doesn't terminate in the first place.
	if parent.Done() == nil {
		return nil
	}

	// killOnce = 0
	if atomic.CompareAndSwapInt32(&killOnce, 0, 3) {
		return nil
	}

	// killOnce = 2
	return ctx.Err()
}
