package util

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/napalu/goopt/types"
)

func ConvertString(value string, data any, arg string, delimiterFunc types.ListDelimiterFunc) error {

	switch t := data.(type) {
	case *string:
		*(t) = value
	case *[]string:
		values := strings.FieldsFunc(value, delimiterFunc)
		*(t) = values
	case *complex64:
		if val, err := strconv.ParseComplex(value, 64); err == nil {
			*(t) = complex64(val)
		} else {
			return fmt.Errorf("invalid complex number: %s", value)
		}
	case *int:
		if num, ok := ParseNumeric(value); !ok || !num.IsInt {
			return fmt.Errorf("invalid integer: %s", value)
		} else if num.IsInt {
			*(t) = int(num.Int)
		} else {
			return fmt.Errorf("integer overflow: %s", value)
		}
	case *[]int:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int, len(values))
		for i, v := range values {
			if num, ok := ParseNumeric(v); !ok || !num.IsInt {
				return fmt.Errorf("invalid integer: %s", v)
			} else if num.IsInt {
				temp[i] = int(num.Int)
			} else {
				return fmt.Errorf("integer overflow: %s", v)
			}
		}
		*(t) = temp
	case *int64:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			*(t) = val
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]int64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 64); err == nil {
				temp[i] = val
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *int32:
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			*(t) = int32(val)
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]int32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 32); err == nil {
				temp[i] = int32(val)
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *int16:
		if val, err := strconv.ParseInt(value, 10, 16); err == nil {
			*(t) = int16(val)
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]int16:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int16, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 16); err == nil {
				temp[i] = int16(val)
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *int8:
		if val, err := strconv.ParseInt(value, 10, 8); err == nil {
			*(t) = int8(val)
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]int8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 8); err == nil {
				temp[i] = int8(val)
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *uint:
		if val, err := strconv.ParseUint(value, 10, strconv.IntSize); err == nil {
			*(t) = uint(val)
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]uint:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, strconv.IntSize); err == nil {
				temp[i] = uint(val)
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *uint64:
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			*(t) = val
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]uint64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 64); err == nil {
				temp[i] = val
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *uint32:
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			*(t) = uint32(val)
		} else {
			return fmt.Errorf("invalid integer: %s", value)
		}
	case *[]uint32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 32); err == nil {
				temp[i] = uint32(val)
			} else {
				return fmt.Errorf("invalid integer: %s", v)
			}
		}
		*(t) = temp
	case *uint16:
		if val, err := strconv.ParseUint(value, 10, 16); err == nil {
			*(t) = uint16(val)
		}
	case *[]uint16:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint16, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 16); err == nil {
				temp[i] = uint16(val)
			}
		}
		*(t) = temp
	case *uint8:
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			*(t) = uint8(val)
		}
	case *[]uint8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 8); err == nil {
				temp[i] = uint8(val)
			}
		}
		*(t) = temp
	case *float64:
		if val, err := strconv.ParseFloat(value, 64); err == nil {
			*(t) = val
		}
	case *[]float64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 64); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *float32:
		if val, err := strconv.ParseFloat(value, 32); err == nil {
			*(t) = float32(val)
		}
	case *[]float32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 32); err == nil {
				temp[i] = float32(val)
			}
		}
		*(t) = temp
	case *bool:
		if val, err := strconv.ParseBool(value); err == nil {
			*(t) = val
		}
	case *[]bool:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]bool, len(values))
		for i, v := range values {
			if val, err := strconv.ParseBool(v); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *time.Time:
		if val, err := dateparse.ParseLocal(value); err == nil {
			*(t) = val
		}
	case *[]time.Time:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]time.Time, len(values))
		for i, v := range values {
			if val, err := dateparse.ParseLocal(v); err == nil {
				temp[i] = val
			}
		}
		*(t) = temp
	case *time.Duration:
		if val, err := time.ParseDuration(value); err == nil {
			*(t) = val
		} else {
			return fmt.Errorf("invalid duration: %s", value)
		}
	case *[]time.Duration:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]time.Duration, len(values))
		for i, v := range values {
			if val, err := time.ParseDuration(v); err == nil {
				temp[i] = val
			} else {
				return fmt.Errorf("invalid duration: %s", v)
			}
		}
		*(t) = temp
	default:
		return fmt.Errorf("%w: unsupported data type %v for argument %s", types.ErrUnsupportedTypeConversion, t, arg)
	}

	return nil
}

func CanConvert(data interface{}, optionType types.OptionType) (bool, error) {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return false, fmt.Errorf("%w: we expect a pointer to a variable", types.ErrUnsupportedTypeConversion)
	}

	supported := true
	var err error
	if optionType == types.Standalone {
		switch data.(type) {
		case *bool:
			return true, nil
		default:
			return false, fmt.Errorf("%w: Standalone fields can only be bound to a boolean variable", types.ErrUnsupportedTypeConversion)
		}
	}

	switch t := data.(type) {
	case *string:
	case *[]string:
	case *complex64:
	case *int:
	case *[]int:
	case *int64:
	case *[]int64:
	case *int32:
	case *[]int32:
	case *int16:
	case *[]int16:
	case *int8:
	case *[]int8:
	case *uint:
	case *[]uint:
	case *uint64:
	case *[]uint64:
	case *uint32:
	case *[]uint32:
	case *uint16:
	case *[]uint16:
	case *uint8:
	case *[]uint8:
	case *float64:
	case *[]float64:
	case *float32:
	case *[]float32:
	case *bool:
	case *[]bool:
	case *time.Time:
	case *[]time.Time:
	case *time.Duration:
	case *[]time.Duration:
	default:
		supported = false
		err = fmt.Errorf("%w: unsupported data type %v", types.ErrUnsupportedTypeConversion, t)
	}

	return supported, err
}
