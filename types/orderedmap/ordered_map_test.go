package orderedmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderedMap(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		om := NewOrderedMap[string, int]()

		// Test Set and Get
		om.Set("one", 1)
		om.Set("two", 2)
		om.Set("three", 3)

		val, exists := om.Get("two")
		assert.True(t, exists)
		assert.Equal(t, 2, val)

		// Test overwrite
		om.Set("two", 22)
		val, exists = om.Get("two")
		assert.True(t, exists)
		assert.Equal(t, 22, val)

		// Test non-existent key
		val, exists = om.Get("four")
		assert.False(t, exists)
		assert.Equal(t, 0, val) // zero value for int
	})

	t.Run("deletion", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		om.Set("one", 1)
		om.Set("two", 2)

		om.Delete("one")
		_, exists := om.Get("one")
		assert.False(t, exists)

		// Delete non-existent key should not panic
		om.Delete("non-existent")

		// Verify remaining key
		val, exists := om.Get("two")
		assert.True(t, exists)
		assert.Equal(t, 2, val)
	})

	t.Run("count", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		assert.Equal(t, 0, om.Count())

		om.Set("one", 1)
		assert.Equal(t, 1, om.Count())

		om.Set("two", 2)
		assert.Equal(t, 2, om.Count())

		om.Delete("one")
		assert.Equal(t, 1, om.Count())
	})

	t.Run("iterator", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		om.Set("one", 1)
		om.Set("two", 2)
		om.Set("three", 3)

		expected := []struct {
			key   string
			value int
		}{
			{"one", 1},
			{"two", 2},
			{"three", 3},
		}

		// Create iterator once - the returned function is a closure that maintains its state
		// between calls, tracking its position in the underlying list
		iter := om.Iterator()
		for idx, key, val := iter(); idx != nil; idx, key, val = iter() {
			assert.Equal(t, expected[*idx].key, *key)
			assert.Equal(t, expected[*idx].value, val)
		}

	})

	t.Run("front to back iteration", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		om.Set("one", 1)
		om.Set("two", 2)
		om.Set("three", 3)

		iter := om.Front()
		require.NotNil(t, iter)
		assert.Equal(t, "one", *iter.Key)
		assert.Equal(t, 1, iter.Value)

		iter = iter.Next()
		require.NotNil(t, iter)
		assert.Equal(t, "two", *iter.Key)
		assert.Equal(t, 2, iter.Value)

		iter = iter.Next()
		require.NotNil(t, iter)
		assert.Equal(t, "three", *iter.Key)
		assert.Equal(t, 3, iter.Value)

		iter = iter.Prev()
		require.NotNil(t, iter)
		assert.Equal(t, "two", *iter.Key)
		assert.Equal(t, 2, iter.Value)

		iter = iter.Next().Next()
		assert.Nil(t, iter)

	})

	t.Run("back to front iteration", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		om.Set("one", 1)
		om.Set("two", 2)
		om.Set("three", 3)

		iter := om.Back()
		require.NotNil(t, iter)
		assert.Equal(t, "three", *iter.Key)
		assert.Equal(t, 3, iter.Value)

		iter = iter.Next()
		require.NotNil(t, iter)
		assert.Equal(t, "two", *iter.Key)
		assert.Equal(t, 2, iter.Value)

		iter = iter.Next()
		require.NotNil(t, iter)
		assert.Equal(t, "one", *iter.Key)
		assert.Equal(t, 1, iter.Value)

		iter = iter.Prev()
		require.NotNil(t, iter)
		assert.Equal(t, "two", *iter.Key)
		assert.Equal(t, 2, iter.Value)

		iter = iter.Next().Next()
		assert.Nil(t, iter)
	})

	t.Run("empty map iteration", func(t *testing.T) {
		om := NewOrderedMap[string, int]()

		assert.Nil(t, om.Front())
		assert.Nil(t, om.Back())

		iter := func() (*int, *string, int) {
			return om.Iterator()()
		}
		idx, key, _ := iter()
		assert.Nil(t, idx)
		assert.Nil(t, key)
	})

	t.Run("complex types", func(t *testing.T) {
		type complexKey struct {
			id int
		}
		type complexValue struct {
			data string
		}

		om := NewOrderedMap[complexKey, complexValue]()
		key1 := complexKey{1}
		val1 := complexValue{"one"}

		om.Set(key1, val1)
		retrieved, exists := om.Get(key1)
		assert.True(t, exists)
		assert.Equal(t, val1, retrieved)
	})

	t.Run("iterator closure behavior", func(t *testing.T) {
		om := NewOrderedMap[string, int]()
		om.Set("one", 1)
		om.Set("two", 2)
		om.Set("three", 3)

		count1 := 0
		iter := om.Iterator()
		for idx, _, _ := iter(); idx != nil; idx, _, _ = iter() {
			count1++
		}
		assert.Equal(t, 3, count1, "reusing iterator closure sees all elements")

		// Multiple iterators can exist independently
		iter1 := om.Iterator()
		iter2 := om.Iterator()

		// First element from each iterator
		idx1, key1, val1 := iter1()
		idx2, key2, val2 := iter2()

		assert.Equal(t, 0, *idx1)
		assert.Equal(t, 0, *idx2)
		assert.Equal(t, "one", *key1)
		assert.Equal(t, "one", *key2)
		assert.Equal(t, 1, val1)
		assert.Equal(t, 1, val2)

		// Advance only iter1
		idx1, key1, val1 = iter1()
		assert.Equal(t, 1, *idx1)
		assert.Equal(t, "two", *key1)
		assert.Equal(t, 2, val1)

		// iter2 still at first element
		idx2, key2, val2 = iter2()
		assert.Equal(t, 1, *idx2)
		assert.Equal(t, "two", *key2)
		assert.Equal(t, 2, val2)
	})
}
