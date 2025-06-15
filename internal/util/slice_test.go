package util

import (
	"reflect"
	"testing"
)

func TestInsertSlice(t *testing.T) {
	tests := []struct {
		name     string
		arr      []int
		pos      int
		elements []int
		want     []int
	}{
		{
			name:     "insert single element in middle",
			arr:      []int{1, 2, 4, 5},
			pos:      2,
			elements: []int{3},
			want:     []int{1, 2, 3, 4, 5},
		},
		{
			name:     "insert multiple elements at start",
			arr:      []int{3, 4},
			pos:      0,
			elements: []int{1, 2},
			want:     []int{1, 2, 3, 4},
		},
		{
			name:     "insert multiple elements at end",
			arr:      []int{1, 2},
			pos:      2,
			elements: []int{3, 4},
			want:     []int{1, 2, 3, 4},
		},
		{
			name:     "insert into empty slice",
			arr:      []int{},
			pos:      0,
			elements: []int{1, 2},
			want:     []int{1, 2},
		},
		{
			name:     "insert empty slice",
			arr:      []int{1, 2},
			pos:      1,
			elements: []int{},
			want:     []int{1, 2},
		},
		{
			name:     "position negative",
			arr:      []int{1, 2, 3},
			pos:      -1,
			elements: []int{0},
			want:     []int{0, 1, 2, 3},
		},
		{
			name:     "position beyond length",
			arr:      []int{1, 2, 3},
			pos:      10,
			elements: []int{4},
			want:     []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InsertSlice(tt.arr, tt.pos, tt.elements...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InsertSlice() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test with string type to verify generic behavior
	strTests := []struct {
		name     string
		arr      []string
		pos      int
		elements []string
		want     []string
	}{
		{
			name:     "insert string in middle",
			arr:      []string{"a", "b", "d"},
			pos:      2,
			elements: []string{"c"},
			want:     []string{"a", "b", "c", "d"},
		},
	}

	for _, tt := range strTests {
		t.Run(tt.name, func(t *testing.T) {
			got := InsertSlice(tt.arr, tt.pos, tt.elements...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InsertSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		name string
		arr  []int
		want []int
	}{
		{
			name: "reverse odd length slice",
			arr:  []int{1, 2, 3, 4, 5},
			want: []int{5, 4, 3, 2, 1},
		},
		{
			name: "reverse even length slice",
			arr:  []int{1, 2, 3, 4},
			want: []int{4, 3, 2, 1},
		},
		{
			name: "reverse single element",
			arr:  []int{1},
			want: []int{1},
		},
		{
			name: "reverse empty slice",
			arr:  []int{},
			want: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := make([]int, len(tt.arr))
			copy(arr, tt.arr)
			Reverse(arr)
			if !reflect.DeepEqual(arr, tt.want) {
				t.Errorf("Reverse() = %v, want %v", arr, tt.want)
			}
		})
	}

	// Test with string type to verify generic behavior
	strTests := []struct {
		name string
		arr  []string
		want []string
	}{
		{
			name: "reverse string slice",
			arr:  []string{"a", "b", "c"},
			want: []string{"c", "b", "a"},
		},
	}

	for _, tt := range strTests {
		t.Run(tt.name, func(t *testing.T) {
			arr := make([]string, len(tt.arr))
			copy(arr, tt.arr)
			Reverse(arr)
			if !reflect.DeepEqual(arr, tt.want) {
				t.Errorf("Reverse() = %v, want %v", arr, tt.want)
			}
		})
	}
}
