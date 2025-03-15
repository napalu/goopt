package util

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/napalu/goopt/i18n"
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
			return i18n.Default().WrapErrorf(types.ErrParseComplex,
				types.ErrParseComplex.Error(), value)
		}
	case *[]complex64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]complex64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseComplex(v, 64); err == nil {
				temp[i] = complex64(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseComplex,
					types.ErrParseComplex.Error(), v)
			}
		}
		*(t) = temp

	case *complex128:
		if val, err := strconv.ParseComplex(value, 128); err == nil {
			*(t) = complex128(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseComplex,
				types.ErrParseComplex.Error(), value)
		}
	case *[]complex128:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]complex128, len(values))
		for i, v := range values {
			if val, err := strconv.ParseComplex(v, 128); err == nil {
				temp[i] = complex128(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseComplex,
					types.ErrParseComplex.Error(), v)
			}
		}
		*(t) = temp
	case *int:
		if num, ok := ParseNumeric(value); !ok || !num.IsInt {
			return i18n.Default().WrapErrorf(types.ErrParseInt,
				types.ErrParseInt.Error(), value)
		} else if num.IsInt {
			*(t) = int(num.Int)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseOverflow,
				types.ErrParseOverflow.Error(), value)
		}
	case *[]int:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int, len(values))
		for i, v := range values {
			if num, ok := ParseNumeric(v); !ok || !num.IsInt {
				return i18n.Default().WrapErrorf(types.ErrParseInt,
					types.ErrParseInt.Error(), v)
			} else if num.IsInt {
				temp[i] = int(num.Int)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseOverflow,
					types.ErrParseOverflow.Error(), v)
			}
		}
		*(t) = temp
	case *int64:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			*(t) = val
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseInt64,
				types.ErrParseInt64.Error(), value)
		}
	case *[]int64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 64); err == nil {
				temp[i] = val
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseInt64,
					types.ErrParseInt64.Error(), v)
			}
		}
		*(t) = temp
	case *int32:
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			*(t) = int32(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseInt32,
				types.ErrParseInt32.Error(), value)
		}
	case *[]int32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 32); err == nil {
				temp[i] = int32(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseInt,
					types.ErrParseInt.Error(), v)
			}
		}
		*(t) = temp
	case *int16:
		if val, err := strconv.ParseInt(value, 10, 16); err == nil {
			*(t) = int16(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseInt,
				types.ErrParseInt.Error(), value)
		}
	case *[]int16:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int16, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 16); err == nil {
				temp[i] = int16(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseInt16,
					types.ErrParseInt16.Error(), v)
			}
		}
		*(t) = temp
	case *int8:
		if val, err := strconv.ParseInt(value, 10, 8); err == nil {
			*(t) = int8(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseInt8,
				types.ErrParseInt8.Error(), value)
		}
	case *[]int8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]int8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseInt(v, 10, 8); err == nil {
				temp[i] = int8(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseInt8,
					types.ErrParseInt8.Error(), v)
			}
		}
		*(t) = temp
	case *uint:
		if val, err := strconv.ParseUint(value, 10, strconv.IntSize); err == nil {
			*(t) = uint(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseUint,
				types.ErrParseUint.Error(), value)
		}
	case *[]uint:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, strconv.IntSize); err == nil {
				temp[i] = uint(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseUint,
					types.ErrParseUint.Error(), v)
			}
		}
		*(t) = temp
	case *uint64:
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			*(t) = val
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseUint64,
				types.ErrParseUint64.Error(), value)
		}
	case *[]uint64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 64); err == nil {
				temp[i] = val
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseUint64,
					types.ErrParseUint64.Error(), v)
			}
		}
		*(t) = temp
	case *uint32:
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			*(t) = uint32(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseUint32,
				types.ErrParseUint32.Error(), value)
		}
	case *[]uint32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 32); err == nil {
				temp[i] = uint32(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseUint32,
					types.ErrParseUint32.Error(), v)
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
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseUint16,
					types.ErrParseUint16.Error(), v)
			}
		}
		*(t) = temp
	case *uint8:
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			*(t) = uint8(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseUint8,
				types.ErrParseUint8.Error(), value)
		}
	case *[]uint8:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]uint8, len(values))
		for i, v := range values {
			if val, err := strconv.ParseUint(v, 10, 8); err == nil {
				temp[i] = uint8(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseUint8,
					types.ErrParseUint8.Error(), v)
			}
		}
		*(t) = temp
	case *float64:
		if val, err := strconv.ParseFloat(value, 64); err == nil {
			*(t) = val
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseFloat64,
				types.ErrParseFloat64.Error(), value)
		}
	case *[]float64:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float64, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 64); err == nil {
				temp[i] = val
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseFloat64,
					types.ErrParseFloat64.Error(), v)
			}
		}
		*(t) = temp
	case *float32:
		if val, err := strconv.ParseFloat(value, 32); err == nil {
			*(t) = float32(val)
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseFloat32,
				types.ErrParseFloat32.Error(), value)
		}
	case *[]float32:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]float32, len(values))
		for i, v := range values {
			if val, err := strconv.ParseFloat(v, 32); err == nil {
				temp[i] = float32(val)
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseFloat32,
					types.ErrParseFloat32.Error(), v)
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
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseBool,
					types.ErrParseBool.Error(), v)
			}
		}
		*(t) = temp
	case *time.Time:
		if val, err := dateparse.ParseLocal(value); err == nil {
			*(t) = val
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseTime,
				types.ErrParseTime.Error(), value)
		}
	case *[]time.Time:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]time.Time, len(values))
		for i, v := range values {
			if val, err := dateparse.ParseLocal(v); err == nil {
				temp[i] = val
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseTime,
					types.ErrParseTime.Error(), v)
			}
		}
		*(t) = temp
	case *time.Duration:
		if val, err := time.ParseDuration(value); err == nil {
			*(t) = val
		} else {
			return i18n.Default().WrapErrorf(types.ErrParseDuration,
				types.ErrParseDuration.Error(), value)
		}
	case *[]time.Duration:
		values := strings.FieldsFunc(value, delimiterFunc)
		temp := make([]time.Duration, len(values))
		for i, v := range values {
			if val, err := time.ParseDuration(v); err == nil {
				temp[i] = val
			} else {
				return i18n.Default().WrapErrorf(types.ErrParseDuration,
					types.ErrParseDuration.Error(), v)
			}
		}
		*(t) = temp
	default:
		return i18n.Default().WrapErrorf(types.ErrUnsupportedTypeConversion,
			types.ErrUnsupportedTypeConversion.Error(), t, arg)
	}

	return nil
}

func CanConvert(data interface{}, optionType types.OptionType) (bool, error) {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return false, i18n.Default().WrapErrorf(types.ErrPointerExpected,
			types.ErrPointerExpected.Error(), optionType)
	}

	supported := true
	var err error
	if optionType == types.Standalone {
		switch data.(type) {
		case *bool:
			return true, nil
		default:
			return false, i18n.Default().WrapErrorf(types.ErrUnsupportedTypeConversion,
				types.ErrFieldBinding.Error(), optionType)
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
		err = i18n.Default().WrapErrorf(types.ErrUnsupportedTypeConversion,
			types.ErrUnsupportedTypeConversion.Error(), t)
	}

	return supported, err
}
