package util

// InsertSlice element T at position pos and returns a slice
func InsertSlice[T any](arr []T, element T, pos int) []T {
	return append(arr[:pos], append([]T{element}, arr[pos:]...)...)
}
