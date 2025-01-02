package queue

import (
	"slices"
	"testing"
)

func TestStackOperations(t *testing.T) {
	q := New[int]()

	// Push items onto the stack
	q.Push(1)
	q.Push(2)
	q.Push(3)

	item, ok := q.Pop()
	if !ok || item != 3 {
		t.Errorf("expected to pop 3 but got %d", item)
	}

	item, ok = q.Peek()
	if !ok || item != 2 {
		t.Errorf("expected Peek to return 2 but got %d", item)
	}

	item, ok = q.Pop()
	if !ok || item != 2 {
		t.Errorf("expected to pop 2 but got %d", item)
	}

	item, ok = q.Pop()
	if !ok || item != 1 {
		t.Errorf("expected to pop 1 but got %d", item)
	}

	_, ok = q.Pop()
	if ok {
		t.Error("expected Pop on empty queue to return false")
	}
}

func TestQueueOperations(t *testing.T) {
	q := New[int]()

	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	item, ok := q.Dequeue()
	if !ok || item != 1 {
		t.Errorf("Expected to dequeue 1 but got %d", item)
	}

	item, ok = q.Dequeue()
	if !ok || item != 2 {
		t.Errorf("Expected to dequeue 2 but got %d", item)
	}

	item, ok = q.Dequeue()
	if !ok || item != 3 {
		t.Errorf("Expected to dequeue 3 but got %d", item)
	}

	_, ok = q.Dequeue()
	if ok {
		t.Error("Expected Dequeue on empty queue to return false")
	}
}

func TestUtilityMethods(t *testing.T) {
	q := New[int]()

	// Check Len
	if q.Len() != 0 {
		t.Errorf("Expected length of new queue to be 0, but got %d", q.Len())
	}

	// Enqueue some items and check Len again
	q.Enqueue(1)
	q.Enqueue(2)

	if q.Len() != 2 {
		t.Errorf("Expected length after enqueueing two items to be 2, but got %d", q.Len())
	}

	item, ok := q.At(0)
	if !ok || item != 1 {
		t.Errorf("Expected At(0) to return 1, but got %d", item)
	}

	item, ok = q.At(1)
	if !ok || item != 2 {
		t.Errorf("Expected At(1) to return 2, but got %d", item)
	}

	q.Clear()
	if q.Len() != 0 {
		t.Errorf("Expected length after clearing the queue to be 0, but got %d", q.Len())
	}
}

func TestIterate(t *testing.T) {
	q := New[int]()
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	expectedItems := []int{1, 2, 3}
	var actualItems []int

	q.ForEach(func(item int, index int) bool {
		actualItems = append(actualItems, item)
		return index < len(expectedItems)-1
	})

	if len(actualItems) != len(expectedItems) {
		t.Errorf("Expected to iterate in order, but got %v", actualItems)
	}

	for i := range expectedItems {
		if actualItems[i] != expectedItems[i] {
			t.Errorf("Expected to iterate in order, but got %v", actualItems[i])
		}
	}

	actualItems = []int{}
	slices.Reverse(expectedItems)

	q.ForEachReverse(func(item int, index int) bool {
		actualItems = append(actualItems, item)
		return index > 0
	})

	if len(actualItems) != len(expectedItems) {
		t.Errorf("Expected %d items after stopping iteration", len(actualItems))
	}

	for i := range expectedItems {
		if actualItems[i] != expectedItems[i] {
			t.Errorf("Expected to iterate in order, but got %v", actualItems[i])
		}
	}

}

func TestEdgeCases(t *testing.T) {
	q := New[int]()

	// Peek on empty queue
	_, ok := q.Peek()
	if ok {
		t.Error("Expected Peek on an empty queue to return false")
	}

	// Dequeue on empty queue
	_, ok = q.Dequeue()
	if ok {
		t.Error("Expected Dequeue on an empty queue to return false")
	}
}

func TestAtInvalidIndices(t *testing.T) {
	q := New[int]()
	q.Push(1)

	_, ok := q.At(-1)
	if ok {
		t.Error("Expected At(-1) to return false")
	}

	_, ok = q.At(1)
	if ok {
		t.Error("Expected At(1) to return false")
	}
}

func TestIterationEarlyTermination(t *testing.T) {
	q := New[int]()
	q.Push(1)
	q.Push(2)
	q.Push(3)

	count := 0
	q.ForEach(func(item int, index int) bool {
		count++
		return false // Stop after first item
	})

	if count != 1 {
		t.Errorf("Expected iteration to stop after 1 item, but processed %d items", count)
	}
}

func TestWithStructs(t *testing.T) {
	type testStruct struct {
		value int
	}

	q := New[testStruct]()
	q.Push(testStruct{1})
	q.Push(testStruct{2})

	item, ok := q.Pop()
	if !ok || item.value != 2 {
		t.Errorf("Expected struct with value 2, got %v", item)
	}
}

func TestZeroValues(t *testing.T) {
	q := New[*int]()

	q.Push(nil)

	item, ok := q.Pop()
	if !ok || item != nil {
		t.Error("Expected to pop nil value")
	}
}
