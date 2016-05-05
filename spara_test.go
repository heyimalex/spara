package spara

import (
	"testing"
)

func TestRunBasic(t *testing.T) {
	input := []int{1, 2, 3}
	output := make([]int, len(input))

	err := Run(3, len(input), func(i int) error {
		output[i] = input[i] * 5
		return nil
	})

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for i, in := range input {
		expected := in * 5
		actual := output[i]
		if expected != actual {
			t.Fatalf("output at index %d wrong: %d != %d", i, expected, actual)
		}
	}
}
