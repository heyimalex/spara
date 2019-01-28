package spara

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunBasic(t *testing.T) {
	tests := []struct {
		name       string
		workers    int
		iterations int
	}{
		{"ZeroIterations", 1, 0},
		{"SingleIteration", 1, 1},
		{"SingleWorker", 1, 10},
		{"EqualWorkersAndIterations", 10, 10},
		{"AbundanceOfWorkers", 100, 10},
		{"CommonCase#1", 3, 10},
		{"CommonCase#2", 5, 10},
		{"CommonCase#3", 7, 10},
		{"CommonCase#4", 9, 10},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if t.Failed() {
					t.Logf("case[%d] workers=%d iterations=%d", i, tc.workers, tc.iterations)
				}
			}()
			subtestRunBasic(t, tc.workers, tc.iterations)
		})
	}
}

func subtestRunBasic(t *testing.T, workers, iterations int) {
	calls := make(map[int]int)
	total := 0
	var mu sync.Mutex

	err := Run(workers, iterations, func(i int) error {
		mu.Lock()
		defer mu.Unlock()
		calls[i] = calls[i] + 1
		total += 1
		return nil
	})

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if iterations != total {
		t.Errorf(
			"number of inputs: %d != number of times called: %d",
			iterations, total,
		)
	}
	for i := 0; i < iterations; i++ {
		count := calls[i]
		if count == 0 {
			t.Errorf("function was not called with index %d", i)
		} else if count > 1 {
			t.Errorf("function called %d times with index %d", count, i)
		}
	}
}

func noopMappingFunc(i int) error {
	return nil
}

func TestRunInputErrors(t *testing.T) {
	if err := Run(0, 10, noopMappingFunc); err != ErrInvalidWorkers {
		t.Errorf("expected calling Run with zero workers to fail: %s", err)
	}
	if err := Run(1, -1, noopMappingFunc); err != ErrInvalidIterations {
		t.Errorf("expected calling Run with negative iterations to fail: %s", err)
	}
}

func TestRunErrorBasic(t *testing.T) {
	const (
		workers    = 5
		iterations = 20
	)
	var count int32
	expectedError := errors.New("")
	err := RunWithContext(context.Background(), workers, iterations, func(ctx context.Context, i int) error {
		atomic.AddInt32(&count, 1)
		if i == workers-1 { // last initial worker
			time.Sleep(time.Millisecond * 10)
			return expectedError
		}
		<-ctx.Done() // wait for context to complete
		return fmt.Errorf("unexpected error, returned from index %d", i)
	})
	if err != expectedError {
		t.Errorf("did not return the expected error: %s", err)
	} else if count != workers {
		t.Errorf("call count: %d != initial worker count: %d", count, workers)
	}
}

func ExampleRun() {
	const workers = 5
	inputs := []int{1, 2, 3, 4, 5}
	outputs := make([]int, len(inputs))

	Run(workers, len(inputs), func(idx int) error {
		outputs[idx] = inputs[idx] * 2
		return nil
	})

	fmt.Println(outputs)
	// Output: [2 4 6 8 10]
}
