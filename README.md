# spara

[Map](https://en.wikipedia.org/wiki/Map_(higher-order_function)) concurrently over slices in go with little boilerplate. Supports early cancellation on error. [![GoDoc](https://godoc.org/github.com/heyimalex/spara?status.svg)](https://godoc.org/github.com/heyimalex/spara)

```
go get github.com/heyimalex/spara
```

**NOTE:** This package requires go 1.7+ as it depends on [context](https://golang.org/pkg/context/).

## Example

```go

inputs := []int{1, 2, 3, 4, 5}      // Input slice
results := make([]int, len(inputs)) // Place to store results
const workers = 5                   // Number of goroutines to spawn

// Run will call the passed function with every index of the input slice.
spara.Run(workers, len(inputs), func(index int) error {
    input := inputs[index]  // The passed index allows us to access the input value
    result := input * 2     // Next we do our "work"
    results[index] = result // And finally we store the result
    return nil
})

fmt.Println(results)
// Output: [2 4 6 8 10]

```

Read more in the [docs](https://godoc.org/github.com/heyimalex/spara).

## Motivation && How it works

At work I've got a lot of code that looks like this:

> Run `function` concurrently for every element in `slice`, and stop processing if there are any errors.

As an example, imagine code that downloads multiple pdfs so that they can be merged together; if one of the downloads fails, we can't complete the merge, and so we shouldn't download any more files.

Before moving to go, this was something that we handled with [GNU Parallel](http://www.gnu.org/software/parallel/). While porting over code I figured go would make this easy, but the lack of generics meant that this sort of concurrency-managing code needed to be written over an over again. More generic implementations tended to abuse reflection or forced the user to write a bunch of type assertions.

But there's a nice workaround for slices: generic iteration is _sort of_ possible in a type-safe way by closing over a collection and passing the number of elements to the mapping function. That sounds... very abstract, so here's an example of how you might use this strategy to get something like functional map.

```go

import "fmt"

func foreach(count int, fn func(int)) {
    for idx := 0; idx < count; idx++ {
        fn(idx)
    }
}

func main() {
    inputs := []int{ 1, 2, 3, 4, 5 }
    outputs := make([]int, len(inputs))

    foreach(len(inputs), func(idx int) {
        outputs[idx] = inputs[idx] * 5
    })

    fmt.Println(outputs) // [5 10 15 20 25]
}

```

This is a whole lot more cumbersome than if go had generics, but it's not so bad. From this, it's not hard to imagine how we could change `foreach` to execute the passed `fn` concurrently by using goroutines and `sync/atomic` to increment the index. If you check out the source you'll see that's exactly what we're doing.
