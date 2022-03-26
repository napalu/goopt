package orderedmap

/*
	Ordered map implementation
	from https://www.tugberkugurlu.com/archive/implementing-ordered-map-in-go-2-0-by-using-generics-with-delete-operation-in-o-1-time-complexity
  	based on
	https://medium.com/swlh/ordered-maps-for-go-using-generics-875ef3816c71
	NOTE: don't rely on the existence of this package in the future if some standard or popular implementation
	emerges.
*/
import (
	"container/list"
)

// Iterator starting at OrderedMap.Front or OrderedMap.Back
type Iterator[K comparable, V any] struct {
	forward bool
	ll      *list.Element
	curr    *list.Element
}

// OrderedMap definition data is stored in insertion order
type OrderedMap[K comparable, V any] struct {
	store map[K]*list.Element
	keys  *list.List
}

type keyValue[K comparable, V any] struct {
	key   K
	value V
}

func newIterator[K comparable, V any](o *OrderedMap[K, V], forward bool) *Iterator[K, V] {
	iter := &Iterator[K, V]{
		forward: forward,
	}

	if o == nil {
		return nil
	}

	if o.keys.Len() == 0 {
		return nil
	}

	if forward {
		iter.ll = o.keys.Front()
	} else {
		iter.ll = o.keys.Back()
	}

	return iter
}

// Next gets the next keyValue or nil when no more values can be iterated on
func (n *Iterator[K, V]) Next() *Iterator[K, V] {
	if n.ll == nil {
		return nil
	}

	if n.forward {
		n.ll = n.ll.Next()
	} else {
		n.ll = n.ll.Prev()
	}

	if n.ll == nil {
		return nil
	}

	return n
}

// Prev gets the previous keyValue or nil when no more values can be iterated on
func (n *Iterator[K, V]) Prev() *Iterator[K, V] {
	if n.ll == nil {
		return nil
	}

	if n.forward {
		n.ll = n.ll.Prev()
	} else {
		n.ll = n.ll.Next()
	}

	if n.ll == nil {
		return nil
	}

	return n
}

// Current returns the current keyValue. If Iterator has no current item, returns an empty keyValue struct
func (n *Iterator[K, V]) Current() func() (*K, V) {
	if n.ll == nil {
		return nil
	}

	keyVal := n.ll.Value.(keyValue[K, V])

	return func() (*K, V) {
		return &keyVal.key, keyVal.value
	}
}

// NewOrderedMap creates a new OrderedMap of type K
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		store: map[K]*list.Element{},
		keys:  list.New(),
	}
}

// Set will store a key-value pair. If the key already exists,
// it will overwrite the existing key-value pair
func (o *OrderedMap[K, V]) Set(key K, val V) {
	var e *list.Element
	if _, exists := o.store[key]; !exists {
		e = o.keys.PushBack(keyValue[K, V]{
			key:   key,
			value: val,
		})
	} else {
		e = o.store[key]
		e.Value = keyValue[K, V]{
			key:   key,
			value: val,
		}
	}
	o.store[key] = e
}

// Get will return the value associated with the key.
// If the key doesn't exist, the second return value will be false.
func (o *OrderedMap[K, V]) Get(key K) (V, bool) {
	val, exists := o.store[key]
	if !exists {
		return *new(V), false
	}
	return val.Value.(keyValue[K, V]).value, true
}

// Iterator is used to loop through the stored key-value pairs.
// The returned anonymous function returns the index, key and value.
func (o *OrderedMap[K, V]) Iterator() func() (*int, *K, V) {
	e := o.keys.Front()
	j := 0
	return func() (_ *int, _ *K, _ V) {
		if e == nil {
			return
		}

		keyVal := e.Value.(keyValue[K, V])
		j++
		e = e.Next()

		return func() *int { v := j - 1; return &v }(), &keyVal.key, keyVal.value
	}
}

// Delete will remove the key and its associated value.
func (o *OrderedMap[K, V]) Delete(key K) {
	e, exists := o.store[key]
	if !exists {
		return
	}

	o.keys.Remove(e)

	delete(o.store, key)
}

// Count returns the count of keys in OrderedMap
func (o *OrderedMap[K, V]) Count() int {
	return o.keys.Len()
}

// Front returns an iterator pointing to the oldest (inserted-first) keyValue
func (o *OrderedMap[K, V]) Front() *Iterator[K, V] {
	return newIterator[K, V](o, true)
}

// Back returns an Iterator pointing to the newest (inserted-last) keyValue
func (o *OrderedMap[K, V]) Back() *Iterator[K, V] {
	return newIterator[K, V](o, false)
}
