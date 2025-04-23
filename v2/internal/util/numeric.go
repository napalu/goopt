package util

import "strconv"

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

type Number struct {
	Int        int64
	Float      float64
	Complex    complex128
	IsInt      bool
	IsFloat    bool
	IsComplex  bool
	IsNegative bool
}

func ParseNumeric(s string) (n Number, ok bool) {
	// Try parsing as int
	if i, err := strconv.ParseInt(s, 0, strconv.IntSize); err == nil {
		n.Int = i
		n.IsInt = true
		n.IsNegative = i < 0
		ok = true
		return
	}

	// Try float if not int
	if f, err := strconv.ParseFloat(s, 64); err == nil && !ok {
		n.Float = f
		n.IsFloat = true
		n.IsNegative = f < 0
		ok = true
		return
	}

	// Try complex if others failed
	if c, err := strconv.ParseComplex(s, 128); err == nil && !ok {
		n.Complex = c
		n.IsComplex = true
		n.IsNegative = real(c) < 0
		ok = true
		return
	}

	return n, ok
}

func Min[T Numeric](x, y T) T {
	if x < y {
		return x
	}
	return y
}
