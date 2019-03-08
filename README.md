# spara [![GoDoc](https://godoc.org/github.com/heyimalex/spara?status.svg)](https://godoc.org/github.com/heyimalex/spara)

Concurrently map over slices in go, with early cancellation on error.

```
go get github.com/heyimalex/spara
```

**NOTE:** This package requires go 1.7+ as it depends on [context](https://golang.org/pkg/context/).

## Usage

You have an array of things, and you want to "process" those things concurrently, stopping early if any of those things fails to process. Use spara like this.

```go
const workers = 5                   // Number of worker goroutines to spawn
inputs := []int{1, 2, 3, 4, 5}      // Your things
results := make([]int, len(inputs)) // Place for your results

// Run will call the passed function with every index of the input slice.
spara.Run(workers, len(inputs), func(index int) error {
    input := inputs[index]  // Access the thing
    result := input * 2     // Process the thing
    results[index] = result // Store the result
    return nil              // Return an error if you like
})

fmt.Println(results)
// Output: [2 4 6 8 10]
```

This package also has support for go's `context.Context`, so your processing function can know when it's time to quit.

```go
parent, cancel := context.WithTimeout(context.Background(), time.Millisecond * 10)
defer cancel()

err := RunWithContext(parent, 5, 50, func(ctx context.Context, idx int) error {
    <-ctx.Done()
    return ctx.Err()
})

fmt.Println(err)
// Output:  context deadline exceeded

```

Read more in the [godoc](https://godoc.org/github.com/heyimalex/spara).
