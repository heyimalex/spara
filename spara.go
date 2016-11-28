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
	return RunWithContext(nil, workers, iterations, wrapped)
}

type MappingFunc func(ctx context.Context, index int) error

// RunWithContext is exactly like Run except that it:
//
// - Accepts a parent context. Completion of this context is tracked via it's
//   Done() channel and will stop iteration early if possible. If completion
//   _does_ cause iteration to stop, the error returned from RunWithContext
//   will be the value of parent.Err(). The passed context may be nil, in
//   which case it defaults to context.Background().
// - Passes a context to the mapping function. This context will be a child of
//   the provided parent context, and will complete either on the first
//   returned error, the parent completing, or all of the work being done.
//
// This method can give very large performance improvements when elements of
// the mapping function support context for early cancallation (eg
// http.Request's WithContext). Imagine a function that downloads multiple
// files in parallel. Using Run, the first error would cause no _new_ files to
// begin downloading, but it still would not return until all of the in
// progress downloads completed. If the in progress downloads are large, that
// could mean you're waiting a very long time for a bunch of data you don't
// actually care about. With early cancellation, these requests would be
// cancelled eagerly, and the function could return much more quickly.
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

	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	var firsterr error
	var once sync.Once
	kill := func(err error) {
		once.Do(func() {
			atomic.StoreInt32(&index, int32(iterations)) // Effectively stops iteration
			cancel()
			firsterr = err
		})
	}

	if parent != context.Background() {
		// Return early if the parent context is already done.
		select {
		case <-parent.Done():
			return parent.Err()
		default:
			break
		}

		// Kill on parent context done. Must be specified _after_ we wrap the
		// parent context WithCancel; if parent never completes, this spawned
		// goroutine wouldn't ever die which is a memory leak.
		go func() {
			<-ctx.Done()
			kill(ctx.Err())
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
	once.Do(func() {})
	return firsterr
}
