package processor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRotationSolverSearch(t *testing.T) {
	// our search function is looking for the first index in the array where the value changes from needle

	t.Run("basic 1", func(t *testing.T) {

		haystack := []int{0, 0, 0, 1, 1, 1}
		l := 1
		r := len(haystack) - 1
		ans, err := search(0, uint64(l), uint64(r), createCheckFun(l, haystack, 0))
		require.NoError(t, err)
		require.Equal(t, 3, int(ans))
	})

	t.Run("basic 2", func(t *testing.T) {

		haystack := []int{0, 0, 0, 1, 1, 1, 2, 2, 2}
		l := 4
		r := len(haystack) - 1
		ans, err := search(0, uint64(l), uint64(r), createCheckFun(l, haystack, 1))
		require.NoError(t, err)
		require.Equal(t, 6, int(ans))
	})

}

func createCheckFun(lowerLimit int, haystack []int, needle int) func(uint64) (int, error) {
	return func(um uint64) (int, error) {
		m := int(um)

		if m < lowerLimit {
			return 0, fmt.Errorf("index out of bounds: too low")
		}
		if len(haystack) <= m {
			return 0, fmt.Errorf("index out of bounds: too high")
		}
		if haystack[m-1] != needle {
			return -1, nil
		}
		if haystack[m] != needle {
			return 0, nil
		}
		return 1, nil
	}
}
