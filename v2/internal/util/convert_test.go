package util

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/napalu/goopt/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestUtil__CanConvert(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		optType   types.OptionType
		want      bool
		wantError bool
	}{
		{
			name:    "string pointer",
			data:    new(string),
			optType: types.Single,
			want:    true,
		},
		{
			name:    "string slice pointer",
			data:    new([]string),
			optType: types.Chained,
			want:    true,
		},
		{
			name:    "int pointer",
			data:    new(int),
			optType: types.Single,
			want:    true,
		},
		{
			name:    "bool pointer with standalone",
			data:    new(bool),
			optType: types.Standalone,
			want:    true,
		},
		{
			name:    "bool pointer with single",
			data:    new(bool),
			optType: types.Single,
			want:    true,
		},
		{
			name:      "non-pointer",
			data:      "string",
			optType:   types.Single,
			want:      false,
			wantError: true,
		},
		{
			name:      "unsupported type",
			data:      new(chan int),
			optType:   types.Single,
			want:      false,
			wantError: true,
		},
		{
			name:      "non-bool standalone",
			data:      new(string),
			optType:   types.Standalone,
			want:      false,
			wantError: true,
		},
		{
			name:    "time.Time pointer",
			data:    new(time.Time),
			optType: types.Single,
			want:    true,
		},
		{
			name:    "time.Duration pointer",
			data:    new(time.Duration),
			optType: types.Single,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanConvert(tt.data, tt.optType)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUtil_ConvertString(t *testing.T) {
	// Default delimiter function
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	tests := []struct {
		name    string
		value   string
		data    interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:  "string",
			value: "test",
			data:  new(string),
			want:  "test",
		},
		{
			name:  "string slice",
			value: "a,b,c",
			data:  new([]string),
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "int",
			value: "42",
			data:  new(int),
			want:  42,
		},
		{
			name:  "int slice",
			value: "1,2,3",
			data:  new([]int),
			want:  []int{1, 2, 3},
		},
		{
			name:  "bool true",
			value: "true",
			data:  new(bool),
			want:  true,
		},
		{
			name:  "bool false",
			value: "false",
			data:  new(bool),
			want:  false,
		},
		{
			name:  "bool slice",
			value: "true,false,true",
			data:  new([]bool),
			want:  []bool{true, false, true},
		},
		{
			name:  "float64",
			value: "3.14",
			data:  new(float64),
			want:  3.14,
		},
		{
			name:  "float64 slice",
			value: "1.1,2.2,3.3",
			data:  new([]float64),
			want:  []float64{1.1, 2.2, 3.3},
		},
		{
			name:  "duration",
			value: "1h30m",
			data:  new(time.Duration),
			want:  90 * time.Minute,
		},
		{
			name:  "duration slice",
			value: "1h,30m,45s",
			data:  new([]time.Duration),
			want:  []time.Duration{time.Hour, 30 * time.Minute, 45 * time.Second},
		},
		{
			name:    "invalid type",
			value:   "test",
			data:    new(chan int),
			wantErr: true,
		},
		{
			name:  "empty string",
			value: "",
			data:  new(string),
			want:  "",
		},
		{
			name:  "whitespace delimited",
			value: "a b c",
			data:  new([]string),
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "pipe delimited",
			value: "a|b|c",
			data:  new([]string),
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "complex64",
			value: "1+2i",
			data:  new(complex64),
			want:  complex64(1 + 2i),
		},
		{
			name:  "int64",
			value: "9223372036854775807", // max int64
			data:  new(int64),
			want:  int64(9223372036854775807),
		},
		{
			name:  "int64 slice",
			value: "9223372036854775807,-9223372036854775808",
			data:  new([]int64),
			want:  []int64{9223372036854775807, -9223372036854775808},
		},
		{
			name:  "int32",
			value: "2147483647", // max int32
			data:  new(int32),
			want:  int32(2147483647),
		},
		{
			name:  "int32 slice",
			value: "2147483647,-2147483648",
			data:  new([]int32),
			want:  []int32{2147483647, -2147483648},
		},
		{
			name:  "int16",
			value: "32767", // max int16
			data:  new(int16),
			want:  int16(32767),
		},
		{
			name:  "int16 slice",
			value: "32767,-32768",
			data:  new([]int16),
			want:  []int16{32767, -32768},
		},
		{
			name:  "int8",
			value: "127", // max int8
			data:  new(int8),
			want:  int8(127),
		},
		{
			name:  "int8 slice",
			value: "127,-128",
			data:  new([]int8),
			want:  []int8{127, -128},
		},
		{
			name:  "uint",
			value: "4294967295",
			data:  new(uint),
			want:  uint(4294967295),
		},
		{
			name:  "uint slice",
			value: "4294967295,0",
			data:  new([]uint),
			want:  []uint{4294967295, 0},
		},
		{
			name:  "uint64",
			value: "18446744073709551615", // max uint64
			data:  new(uint64),
			want:  uint64(18446744073709551615),
		},
		{
			name:  "uint64 slice",
			value: "18446744073709551615,0",
			data:  new([]uint64),
			want:  []uint64{18446744073709551615, 0},
		},
		{
			name:  "uint32",
			value: "4294967295", // max uint32
			data:  new(uint32),
			want:  uint32(4294967295),
		},
		{
			name:  "uint32 slice",
			value: "4294967295,0",
			data:  new([]uint32),
			want:  []uint32{4294967295, 0},
		},
		{
			name:  "uint16",
			value: "65535", // max uint16
			data:  new(uint16),
			want:  uint16(65535),
		},
		{
			name:  "uint16 slice",
			value: "65535,0",
			data:  new([]uint16),
			want:  []uint16{65535, 0},
		},
		{
			name:  "uint8",
			value: "255", // max uint8
			data:  new(uint8),
			want:  uint8(255),
		},
		{
			name:  "uint8 slice",
			value: "255,0",
			data:  new([]uint8),
			want:  []uint8{255, 0},
		},
		{
			name:  "float32",
			value: "3.4028235e+38", // max float32
			data:  new(float32),
			want:  float32(3.4028235e+38),
		},
		{
			name:  "float32 slice",
			value: "3.4028235e+38,-3.4028235e+38",
			data:  new([]float32),
			want:  []float32{3.4028235e+38, -3.4028235e+38},
		},
		{
			name:  "time.Time",
			value: "2024-01-01T12:00:00Z",
			data:  new(time.Time),
			want:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name:  "time.Time slice",
			value: "2024-01-01T12:00:00Z,2024-12-31T23:59:59Z",
			data:  new([]time.Time),
			want: []time.Time{
				time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			},
		},
		{
			name:    "hex to int",
			value:   "0x1a",
			data:    new(int),
			want:    26,
			wantErr: false,
		},
		{
			name:    "hex to uint (invalid)",
			value:   "0x1a",
			data:    new(uint),
			want:    0,
			wantErr: true,
		},
		{
			name:    "binary to int",
			value:   "0b1010",
			data:    new(int),
			want:    10,
			wantErr: false,
		},
		{
			name:    "negative to uint (invalid)",
			value:   "-5",
			data:    new(uint),
			want:    0,
			wantErr: true,
		},
		// Test error cases for all numeric types
		{
			name:    "invalid int8",
			value:   "256", // overflow
			data:    new(int8),
			wantErr: true,
		},
		{
			name:    "invalid int8 slice",
			value:   "127,256", // second value overflows
			data:    new([]int8),
			wantErr: true,
		},
		{
			name:    "invalid int16",
			value:   "65536", // overflow
			data:    new(int16),
			wantErr: true,
		},
		{
			name:    "invalid int16 slice",
			value:   "32767,65536", // second value overflows
			data:    new([]int16),
			wantErr: true,
		},
		{
			name:    "invalid int32",
			value:   "2147483648", // overflow
			data:    new(int32),
			wantErr: true,
		},
		{
			name:    "invalid int32 slice",
			value:   "2147483647,2147483648", // second value overflows
			data:    new([]int32),
			wantErr: true,
		},
		{
			name:    "invalid int64",
			value:   "not-a-number",
			data:    new(int64),
			wantErr: true,
		},
		{
			name:    "invalid int64 slice",
			value:   "123,not-a-number",
			data:    new([]int64),
			wantErr: true,
		},
		{
			name:    "invalid int",
			value:   "not-a-number",
			data:    new(int),
			wantErr: true,
		},
		{
			name:    "invalid int slice",
			value:   "1,2,not-a-number",
			data:    new([]int),
			wantErr: true,
		},
		{
			name:    "invalid uint8",
			value:   "256", // overflow
			data:    new(uint8),
			wantErr: true,
		},
		{
			name:    "invalid uint8 slice",
			value:   "255,256", // second value overflows
			data:    new([]uint8),
			wantErr: true,
		},
		{
			name:    "invalid uint16",
			value:   "65536", // overflow
			data:    new(uint16),
			wantErr: true,
		},
		{
			name:    "invalid uint16 slice",
			value:   "65535,65536", // second value overflows
			data:    new([]uint16),
			wantErr: true,
		},
		{
			name:    "invalid uint32",
			value:   "4294967296", // overflow
			data:    new(uint32),
			wantErr: true,
		},
		{
			name:    "invalid uint32 slice",
			value:   "4294967295,4294967296", // second value overflows
			data:    new([]uint32),
			wantErr: true,
		},
		{
			name:    "invalid uint64",
			value:   "not-a-number",
			data:    new(uint64),
			wantErr: true,
		},
		{
			name:    "invalid uint64 slice",
			value:   "123,not-a-number",
			data:    new([]uint64),
			wantErr: true,
		},
		{
			name:    "invalid uint",
			value:   "not-a-number",
			data:    new(uint),
			wantErr: true,
		},
		{
			name:    "invalid uint slice",
			value:   "1,2,not-a-number",
			data:    new([]uint),
			wantErr: true,
		},
		{
			name:    "invalid float32",
			value:   "not-a-float",
			data:    new(float32),
			wantErr: true,
		},
		{
			name:    "invalid float32 slice",
			value:   "1.1,not-a-float",
			data:    new([]float32),
			wantErr: true,
		},
		{
			name:    "invalid float64",
			value:   "not-a-float",
			data:    new(float64),
			wantErr: true,
		},
		{
			name:    "invalid float64 slice",
			value:   "1.1,not-a-float",
			data:    new([]float64),
			wantErr: true,
		},
		{
			name:    "invalid bool",
			value:   "not-a-bool",
			data:    new(bool),
			wantErr: true,
		},
		{
			name:    "invalid bool slice",
			value:   "true,not-a-bool",
			data:    new([]bool),
			wantErr: true,
		},
		{
			name:    "invalid time",
			value:   "not-a-time",
			data:    new(time.Time),
			wantErr: true,
		},
		{
			name:    "invalid time slice",
			value:   "2024-01-01T12:00:00Z,not-a-time",
			data:    new([]time.Time),
			wantErr: true,
		},
		{
			name:    "invalid duration",
			value:   "not-a-duration",
			data:    new(time.Duration),
			wantErr: true,
		},
		{
			name:    "invalid duration slice",
			value:   "1h,not-a-duration",
			data:    new([]time.Duration),
			wantErr: true,
		},
		{
			name:    "invalid complex64",
			value:   "not-a-complex",
			data:    new(complex64),
			wantErr: true,
		},
		{
			name:  "complex64 slice",
			value: "1+2i,3+4i",
			data:  new([]complex64),
			want:  []complex64{1 + 2i, 3 + 4i},
		},
		{
			name:    "invalid complex64 slice",
			value:   "1+2i,not-a-complex",
			data:    new([]complex64),
			wantErr: true,
		},
		{
			name:  "complex128",
			value: "5+6i",
			data:  new(complex128),
			want:  complex128(5 + 6i),
		},
		{
			name:  "complex128 slice",
			value: "5+6i,7+8i",
			data:  new([]complex128),
			want:  []complex128{5 + 6i, 7 + 8i},
		},
		{
			name:    "invalid complex128",
			value:   "not-a-complex",
			data:    new(complex128),
			wantErr: true,
		},
		{
			name:    "invalid complex128 slice",
			value:   "5+6i,not-a-complex",
			data:    new([]complex128),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ConvertString(tt.value, tt.data, tt.name, delimiter)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			actual := reflect.ValueOf(tt.data).Elem().Interface()
			assert.Equal(t, tt.want, actual)
		})
	}
}

// TestUtil_ConvertString_Append tests the append mode of ConvertString
func TestUtil_ConvertString_Append(t *testing.T) {
	// Default delimiter function
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	t.Run("append to string slice", func(t *testing.T) {
		data := []string{"existing"}
		err := ConvertString("new1,new2", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []string{"existing", "new1", "new2"}, data)
	})

	t.Run("append to int slice", func(t *testing.T) {
		data := []int{1, 2}
		err := ConvertString("3,4", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3, 4}, data)
	})

	t.Run("append to int8 slice", func(t *testing.T) {
		data := []int8{1}
		err := ConvertString("2,3", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []int8{1, 2, 3}, data)
	})

	t.Run("append to int16 slice", func(t *testing.T) {
		data := []int16{100}
		err := ConvertString("200,300", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []int16{100, 200, 300}, data)
	})

	t.Run("append to int32 slice", func(t *testing.T) {
		data := []int32{1000}
		err := ConvertString("2000,3000", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []int32{1000, 2000, 3000}, data)
	})

	t.Run("append to int64 slice", func(t *testing.T) {
		data := []int64{10000}
		err := ConvertString("20000,30000", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []int64{10000, 20000, 30000}, data)
	})

	t.Run("append to uint slice", func(t *testing.T) {
		data := []uint{1}
		err := ConvertString("2,3", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []uint{1, 2, 3}, data)
	})

	t.Run("append to uint8 slice", func(t *testing.T) {
		data := []uint8{10}
		err := ConvertString("20,30", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []uint8{10, 20, 30}, data)
	})

	t.Run("append to uint16 slice", func(t *testing.T) {
		data := []uint16{100}
		err := ConvertString("200,300", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []uint16{100, 200, 300}, data)
	})

	t.Run("append to uint32 slice", func(t *testing.T) {
		data := []uint32{1000}
		err := ConvertString("2000,3000", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []uint32{1000, 2000, 3000}, data)
	})

	t.Run("append to uint64 slice", func(t *testing.T) {
		data := []uint64{10000}
		err := ConvertString("20000,30000", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []uint64{10000, 20000, 30000}, data)
	})

	t.Run("append to float32 slice", func(t *testing.T) {
		data := []float32{1.1}
		err := ConvertString("2.2,3.3", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []float32{1.1, 2.2, 3.3}, data)
	})

	t.Run("append to float64 slice", func(t *testing.T) {
		data := []float64{1.1}
		err := ConvertString("2.2,3.3", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []float64{1.1, 2.2, 3.3}, data)
	})

	t.Run("append to bool slice", func(t *testing.T) {
		data := []bool{true}
		err := ConvertString("false,true", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []bool{true, false, true}, data)
	})

	t.Run("append to time slice", func(t *testing.T) {
		existing := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		data := []time.Time{existing}
		err := ConvertString("2024-12-31T23:59:59Z", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Len(t, data, 2)
		assert.Equal(t, existing, data[0])
	})

	t.Run("append to duration slice", func(t *testing.T) {
		data := []time.Duration{time.Hour}
		err := ConvertString("30m,45s", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []time.Duration{time.Hour, 30 * time.Minute, 45 * time.Second}, data)
	})

	t.Run("append to complex64 slice", func(t *testing.T) {
		data := []complex64{1 + 2i}
		err := ConvertString("3+4i,5+6i", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []complex64{1 + 2i, 3 + 4i, 5 + 6i}, data)
	})

	t.Run("append to complex128 slice", func(t *testing.T) {
		data := []complex128{1 + 2i}
		err := ConvertString("3+4i,5+6i", &data, "test", delimiter, true)
		assert.NoError(t, err)
		assert.Equal(t, []complex128{1 + 2i, 3 + 4i, 5 + 6i}, data)
	})

	// Test error cases during append
	t.Run("append invalid int to slice", func(t *testing.T) {
		data := []int{1}
		err := ConvertString("not-a-number", &data, "test", delimiter, true)
		assert.Error(t, err)
		// Original data should be unchanged on error
		assert.Equal(t, []int{1}, data)
	})

	t.Run("append invalid bool to slice", func(t *testing.T) {
		data := []bool{true}
		err := ConvertString("not-a-bool", &data, "test", delimiter, true)
		assert.Error(t, err)
		assert.Equal(t, []bool{true}, data)
	})
}

// BenchmarkStdLib provides comparison against standard library functions
func BenchmarkStdLib(b *testing.B) {
	b.Run("strconv-int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = strconv.Atoi("12345")
		}
	})

	b.Run("strings-split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = strings.Split("a,b,c", ",")
		}
	})

	b.Run("time-parse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = time.Parse(time.RFC3339, "2024-01-01T12:00:00Z")
		}
	})
}

// BenchmarkCanConvert benchmarks type checking
func BenchmarkCanConvert(b *testing.B) {
	b.Run("simple-type", func(b *testing.B) {
		data := new(string)
		for i := 0; i < b.N; i++ {
			_, _ = CanConvert(data, types.Single)
		}
	})

	b.Run("slice-type", func(b *testing.B) {
		data := new([]string)
		for i := 0; i < b.N; i++ {
			_, _ = CanConvert(data, types.Chained)
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}
	b.Run("slice-growing", func(b *testing.B) {
		var ss []string
		input := strings.Repeat("value,", 1000) + "value"
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = ConvertString(input, &ss, "test", delimiter)
		}
	})
}

// BenchmarkConvertString benchmarks string conversion
func BenchmarkConvertString(b *testing.B) {
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	// Reduce default iterations for slower operations
	b.Run("string-simple", func(b *testing.B) {
		var s string
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = ConvertString("test-value", &s, "test", delimiter)
		}
	})
}

// Separate benchmark for numeric types
func BenchmarkConvertNumeric(b *testing.B) {
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	b.Run("int-conversion", func(b *testing.B) {
		var j int
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ConvertString("12345", &j, "test", delimiter)
		}
	})
}

// Separate benchmark for slices
func BenchmarkConvertSlices(b *testing.B) {
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	b.Run("slice-small", func(b *testing.B) {
		var ss []string
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ConvertString("a,b,c", &ss, "test", delimiter)
		}
	})

	b.Run("slice-medium", func(b *testing.B) {
		var ss []string
		input := strings.Repeat("value,", 10) + "value"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ConvertString(input, &ss, "test", delimiter)
		}
	})
}

// Separate benchmark for time types
func BenchmarkConvertTime(b *testing.B) {
	delimiter := func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	}

	b.Run("time-parse", func(b *testing.B) {
		var t time.Time
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ConvertString("2024-01-01T12:00:00Z", &t, "test", delimiter)
		}
	})

	b.Run("duration-parse", func(b *testing.B) {
		var d time.Duration
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ConvertString("1h30m", &d, "test", delimiter)
		}
	})
}
