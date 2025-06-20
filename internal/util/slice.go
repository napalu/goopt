package util

// InsertSlice inserts item(s) T at position pos and returns a slice
func InsertSlice[T any](arr []T, pos int, element ...T) []T {
	if pos < 0 {
		pos = 0
	}
	if pos > len(arr) {
		pos = len(arr)
	}

	return append(arr[:pos], append(element, arr[pos:]...)...)
}

// Reverse reverses the slice in place
func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
