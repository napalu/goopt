package util

// InsertSlice inserts item(s) T at position pos and returns a slice
func InsertSlice[T any](arr []T, pos int, element ...T) []T {
	return append(arr[:pos], append(element, arr[pos:]...)...)
}
