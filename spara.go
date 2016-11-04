// Package spara provides a couple of functions for concurrent mapping over elements of a slice with
// early cancellation.
package spara

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidWorkers     = errors.New("spara: invalid number of workers")
	ErrInvalidIterations  = errors.New("spara: invalid number of iterations")
)

func Run(workers int, iterations int, fn func(int) error) error {
	wrapped := func(_ Context, idx int) error {
		return fn(idx)
	}
	return RunWithContext(workers, iterations, wrapped)
}

func RunWithContext(workers int, iterations int, fn func(Context, int) error) error {

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

	var index int32 = -1
	nextIndex := func() int {
		return int(atomic.AddInt32(&index, 1))
	}

	ctx := &context{cancelchan: make(chan struct{})}

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := nextIndex(); j < iterations; j = nextIndex() {
				if ctx.IsCanceled() {
					return
				}
				if err := fn(ctx, j); err != nil {
					ctx.cancel(err)
					return
				}
			}
		}()
	}

	wg.Wait()
	return ctx.err
}

// Context exposes information about early cancellation to the work function. This allows work
// functions that are long or have multiple stages to avoid doing unnecessary work by returning
// early when the process has been canceled.
//
// Imagine a work function that downloads files. If any of the downloads fail, no more downloads
// will be initiated, but all of the currently in-progress downloads will continue to run until
// completion. Since the `Run` functions don't return until all workers have completed, you'll be
// waiting on a bunch of requests to complete that you really don't care about.
//
// A better way to handle this would be using Context. When you create the `net/http.Request` to
// send, you can set its `Cancel` field to `Context.Canceled()`. Then whenever the process
// encounters an error, any in progress downloads will be terminated immediately.
type Context interface {
	// Returns whether or not the process has encountered an error.
	IsCanceled() bool
	// Returns a channel that will be closed when the process has encountered an error.
	Canceled() <-chan struct{}
}

type context struct {
	cancelswitch uint32
	cancelchan   chan struct{}
	err          error
}

func (ctx *context) cancel(err error) {
	if atomic.CompareAndSwapUint32(&ctx.cancelswitch, 0, 1) {
		ctx.err = err
		close(ctx.cancelchan)
	}
}

func (ctx *context) IsCanceled() bool {
	return atomic.LoadUint32(&ctx.cancelswitch) == 1
}

func (ctx *context) Canceled() <-chan struct{} {
	return ctx.cancelchan
}
