package util

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/napalu/goopt/v2/errs"
	"github.com/napalu/goopt/v2/types"
)

func ConvertString(value string, data any, arg string, delimiterFunc types.ListDelimiterFunc, useAppend ...bool) error {
	doAppend := false
	if len(useAppend) > 0 {
		doAppend = useAppend[0]
	}
	switch t := data.(type) {
	case *string:
		*(t) = value
	case *[]string:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			*(t) = values
		} else {
			*t = append(*t, values...)
		}
	case *complex64:
		val, err := strconv.ParseComplex(value, 64)
		if err != nil {
			return errs.ErrParseComplex.WithArgs(value).Wrap(err)
		}
		*(t) = complex64(val)
	case *[]complex64:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]complex64, len(values))
			for i, v := range values {
				val, err := strconv.ParseComplex(v, 64)
				if err != nil {
					return errs.ErrParseComplex.WithArgs(v).Wrap(err)
				}
				temp[i] = complex64(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseComplex(v, 64)
				if err != nil {
					return errs.ErrParseComplex.WithArgs(v).Wrap(err)
				}
				*t = append(*t, complex64(val))
			}
		}
	case *complex128:
		val, err := strconv.ParseComplex(value, 128)
		if err != nil {
			return errs.ErrParseComplex.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]complex128:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]complex128, len(values))
			for i, v := range values {
				val, err := strconv.ParseComplex(v, 128)
				if err != nil {
					return errs.ErrParseComplex.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseComplex(v, 128)
				if err != nil {
					return errs.ErrParseComplex.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	case *int:
		val, err := strconv.ParseInt(value, 0, strconv.IntSize)
		if err != nil {
			var numErr *strconv.NumError
			if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
				return errs.ErrParseOverflow.WithArgs(value).Wrap(err)
			}
			return errs.ErrParseInt.WithArgs(value).Wrap(err)
		}
		*(t) = int(val)
	case *[]int:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]int, len(values))
			for i, v := range values {
				val, err := strconv.ParseInt(v, 0, strconv.IntSize) // Directly use native size
				if err != nil {
					var numErr *strconv.NumError
					if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
						return errs.ErrParseOverflow.WithArgs(value).Wrap(err)
					}
					return errs.ErrParseInt.WithArgs(value).Wrap(err)
				}
				temp[i] = int(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseInt(v, 0, strconv.IntSize) // Directly use native size
				if err != nil {
					var numErr *strconv.NumError
					if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
						return errs.ErrParseOverflow.WithArgs(value).Wrap(err)
					}
					return errs.ErrParseInt.WithArgs(value).Wrap(err)
				}
				*t = append(*t, int(val))
			}
		}
	case *int64:
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return errs.ErrParseInt64.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]int64:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]int64, len(values))
			for i, v := range values {
				val, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return errs.ErrParseInt64.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return errs.ErrParseInt64.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	case *int32:
		val, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return errs.ErrParseInt32.WithArgs(value).Wrap(err)
		}
		*(t) = int32(val)
	case *[]int32:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]int32, len(values))
			for i, v := range values {
				val, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					return errs.ErrParseInt32.WithArgs(v).Wrap(err)
				}
				temp[i] = int32(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					return errs.ErrParseInt32.WithArgs(v).Wrap(err)
				}
				*t = append(*t, int32(val))
			}
		}
	case *int16:
		val, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			return errs.ErrParseInt16.WithArgs(value).Wrap(err)
		}
		*(t) = int16(val)
	case *[]int16:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]int16, len(values))
			for i, v := range values {
				val, err := strconv.ParseInt(v, 10, 16)
				if err != nil {
					return errs.ErrParseInt16.WithArgs(v).Wrap(err)
				}
				temp[i] = int16(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseInt(v, 10, 16)
				if err != nil {
					return errs.ErrParseInt16.WithArgs(v).Wrap(err)
				}
				*t = append(*t, int16(val))
			}
		}
	case *int8:
		val, err := strconv.ParseInt(value, 10, 8)
		if err != nil {
			return errs.ErrParseInt8.WithArgs(value).Wrap(err)
		}
		*(t) = int8(val)
	case *[]int8:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]int8, len(values))
			for i, v := range values {
				val, err := strconv.ParseInt(v, 10, 8)
				if err != nil {
					return errs.ErrParseInt8.WithArgs(v).Wrap(err)
				}
				temp[i] = int8(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseInt(v, 10, 8)
				if err != nil {
					return errs.ErrParseInt8.WithArgs(v).Wrap(err)
				}
				*t = append(*t, int8(val))
			}
		}
	case *uint:
		val, err := strconv.ParseUint(value, 10, strconv.IntSize)
		if err != nil {
			return errs.ErrParseUint.WithArgs(value).Wrap(err)
		}
		*(t) = uint(val)
	case *[]uint:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]uint, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(v, 10, strconv.IntSize)
				if err != nil {
					return errs.ErrParseUint.WithArgs(v).Wrap(err)
				}
				temp[i] = uint(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseUint(v, 10, strconv.IntSize)
				if err != nil {
					return errs.ErrParseUint.WithArgs(v).Wrap(err)
				}
				*t = append(*t, uint(val))
			}
		}
	case *uint64:
		val, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return errs.ErrParseUint64.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]uint64:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]uint64, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return errs.ErrParseUint64.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return errs.ErrParseUint64.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	case *uint32:
		val, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return errs.ErrParseUint32.WithArgs(value).Wrap(err)
		}
		*(t) = uint32(val)
	case *[]uint32:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]uint32, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(v, 10, 32)
				if err != nil {
					return errs.ErrParseUint32.WithArgs(v).Wrap(err)
				}
				temp[i] = uint32(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseUint(v, 10, 32)
				if err != nil {
					return errs.ErrParseUint32.WithArgs(v).Wrap(err)
				}
				*t = append(*t, uint32(val))
			}
		}
	case *uint16:
		val, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return errs.ErrParseUint16.WithArgs(value).Wrap(err)
		}
		*(t) = uint16(val)
	case *[]uint16:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]uint16, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(v, 10, 16)
				if err != nil {
					return errs.ErrParseUint16.WithArgs(v).Wrap(err)
				}
				temp[i] = uint16(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseUint(v, 10, 16)
				if err != nil {
					return errs.ErrParseUint16.WithArgs(v).Wrap(err)
				}
				*t = append(*t, uint16(val))
			}
		}
	case *uint8:
		val, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return errs.ErrParseUint8.WithArgs(value).Wrap(err)
		}
		*(t) = uint8(val)
	case *[]uint8:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]uint8, len(values))
			for i, v := range values {
				val, err := strconv.ParseUint(v, 10, 8)
				if err != nil {
					return errs.ErrParseUint8.WithArgs(v).Wrap(err)
				}
				temp[i] = uint8(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseUint(v, 10, 8)
				if err != nil {
					return errs.ErrParseUint8.WithArgs(v).Wrap(err)
				}
				*t = append(*t, uint8(val))
			}
		}
	case *float64:
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errs.ErrParseFloat64.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]float64:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]float64, len(values))
			for i, v := range values {
				val, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return errs.ErrParseFloat64.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return errs.ErrParseFloat64.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	case *float32:
		val, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return errs.ErrParseFloat32.WithArgs(value).Wrap(err)
		}
		*(t) = float32(val)
	case *[]float32:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]float32, len(values))
			for i, v := range values {
				val, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return errs.ErrParseFloat32.WithArgs(v).Wrap(err)
				}
				temp[i] = float32(val)
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := strconv.ParseFloat(v, 32)
				if err != nil {
					return errs.ErrParseFloat32.WithArgs(v).Wrap(err)
				}
				*t = append(*t, float32(val))
			}
		}
	case *bool:
		val, err := strconv.ParseBool(value)
		if err != nil {
			return errs.ErrParseBool.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]bool:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]bool, len(values))
			for i, v := range values {
				if val, err := strconv.ParseBool(v); err == nil {
					temp[i] = val
				} else {
					return errs.ErrParseBool.WithArgs(v)
				}
			}
			*(t) = temp
		} else {
			for _, v := range values {
				if val, err := strconv.ParseBool(v); err == nil {
					*t = append(*t, val)

				} else {
					return errs.ErrParseBool.WithArgs(v)
				}
			}
		}
	case *time.Time:
		val, err := dateparse.ParseLocal(value)
		if err != nil {
			return errs.ErrParseTime.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]time.Time:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]time.Time, len(values))
			for i, v := range values {
				val, err := dateparse.ParseLocal(v)
				if err != nil {
					return errs.ErrParseTime.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := dateparse.ParseLocal(v)
				if err != nil {
					return errs.ErrParseTime.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	case *time.Duration:
		val, err := time.ParseDuration(value)
		if err != nil {
			return errs.ErrParseDuration.WithArgs(value).Wrap(err)
		}
		*(t) = val
	case *[]time.Duration:
		values := strings.FieldsFunc(value, delimiterFunc)
		if !doAppend {
			temp := make([]time.Duration, len(values))
			for i, v := range values {
				val, err := time.ParseDuration(v)
				if err != nil {
					return errs.ErrParseDuration.WithArgs(v).Wrap(err)
				}
				temp[i] = val
			}
			*(t) = temp
		} else {
			for _, v := range values {
				val, err := time.ParseDuration(v)
				if err != nil {
					return errs.ErrParseDuration.WithArgs(v).Wrap(err)
				}
				*t = append(*t, val)
			}
		}
	default:
		return errs.ErrUnsupportedTypeConversion.WithArgs(t, arg)
	}

	return nil
}

func CanConvert(data interface{}, optionType types.OptionType) (bool, error) {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return false, errs.ErrPointerExpected.WithArgs(optionType)
	}

	supported := true
	var err error
	if optionType == types.Standalone {
		switch data.(type) {
		case *bool:
			return true, nil
		default:
			return false, errs.ErrFieldBinding.WithArgs(optionType)
		}
	}

	switch t := data.(type) {
	case *string:
	case *[]string:
	case *complex64:
	case *[]complex64:
	case *complex128:
	case *[]complex128:
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
		err = errs.ErrUnsupportedTypeConversion.WithArgs(t)
	}

	return supported, err
}
