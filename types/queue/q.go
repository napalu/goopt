package queue

// Q is a generic stack/queue structure that supports both stack and queue operations.
type Q[T any] struct {
	items []T
}

func New[T any]() *Q[T] {
	return &Q[T]{}
}

// Stack Operations

// Push adds an item to the top of the stack (stack behavior)
func (q *Q[T]) Push(item T) {
	q.items = append(q.items, item)
}

// Pop removes and returns the top item from the stack (stack behavior)
func (q *Q[T]) Pop() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[len(q.items)-1]
	q.items = q.items[:len(q.items)-1]
	return item, true
}

// Peek returns the top item from the stack without removing it
func (q *Q[T]) Peek() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	return q.items[len(q.items)-1], true
}

// Queue Operations

// Enqueue adds an item to the end of the queue (queue behavior)
func (q *Q[T]) Enqueue(item T) {
	q.items = append(q.items, item)
}

// Dequeue removes and returns the first item from the queue (queue behavior)
func (q *Q[T]) Dequeue() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// Utility Methods

// Len returns the number of items in the Q
func (q *Q[T]) Len() int {
	return len(q.items)
}

// At returns the item at a specific index
func (q *Q[T]) At(index int) (T, bool) {
	if index < 0 || index >= len(q.items) {
		var zero T
		return zero, false
	}
	return q.items[index], true
}

// Iterate allows you to iterate over the stack (from top to bottom) or queue (from front to back)
func (q *Q[T]) Iterate(callback func(item T, index int) bool) {
	for i := len(q.items) - 1; i >= 0; i-- {
		if !callback(q.items[i], i) {
			break
		}
	}
}

func (q *Q[T]) Clear() {
	q.items = q.items[:0]
}
