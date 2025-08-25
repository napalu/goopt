package util

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUniqueID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		id1 := UniqueID("test")
		id2 := UniqueID("test")

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("with prefix", func(t *testing.T) {
		prefix := "test-"
		id := UniqueID(prefix)

		assert.True(t, strings.HasPrefix(id, prefix))
		assert.Greater(t, len(id), len(prefix))
	})

	t.Run("concurrent generation", func(t *testing.T) {
		const numGoroutines = 10
		const numIDs = 100

		var wg sync.WaitGroup
		idChan := make(chan string, numGoroutines*numIDs)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numIDs; j++ {
					idChan <- UniqueID("concurrent-")
				}
			}()
		}

		wg.Wait()
		close(idChan)

		// Check all IDs are unique
		seen := make(map[string]bool)
		for id := range idChan {
			assert.False(t, seen[id], "duplicate ID found: %s", id)
			seen[id] = true
		}

		assert.Equal(t, numGoroutines*numIDs, len(seen))
	})

	t.Run("empty prefix", func(t *testing.T) {
		id := UniqueID("")
		assert.NotEmpty(t, id)
	})
}
