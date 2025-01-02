package util

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

func Min[T Numeric](x, y T) T {
	if x < y {
		return x
	}
	return y
}
