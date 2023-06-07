package archway

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorageKey(t *testing.T) {
	s := getKey(STORAGEKEY__Commitments)
	expected := fmt.Sprintf("000b%x", STORAGEKEY__Commitments)
	assert.Equal(t, expected, s)
}
