package auth

import (
	"fmt"
	"reflect"
	"strconv"
)

func strParser(kind reflect.Kind) func(string) (any, error) {
	parseInt := func(str string, bitSize int) (any, error) {
		num, err := strconv.ParseInt(str, 10, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 8:
			return int8(num), nil
		case 16:
			return int16(num), nil
		case 32:
			return int32(num), nil
		case 64:
			return num, nil
		default:
			return int(num), nil
		}
	}

	parseUint := func(str string, bitSize int) (any, error) {
		num, err := strconv.ParseUint(str, 10, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 8:
			return uint8(num), nil
		case 16:
			return uint16(num), nil
		case 32:
			return uint32(num), nil
		case 64:
			return num, nil
		default:
			return uint(num), nil
		}
	}

	switch kind {
	case reflect.String:
		return func(str string) (any, error) {
			return str, nil
		}
	case reflect.Int:
		return func(str string) (any, error) {
			return parseInt(str, strconv.IntSize)
		}
	case reflect.Int8:
		return func(str string) (any, error) {
			return parseInt(str, 8)
		}
	case reflect.Int16:
		return func(str string) (any, error) {
			return parseInt(str, 16)
		}
	case reflect.Int32:
		return func(str string) (any, error) {
			return parseInt(str, 32)
		}
	case reflect.Int64:
		return func(str string) (any, error) {
			return parseInt(str, 64)
		}
	case reflect.Uint:
		return func(str string) (any, error) {
			return parseUint(str, strconv.IntSize)
		}
	case reflect.Uint8:
		return func(str string) (any, error) {
			return parseUint(str, 8)
		}
	case reflect.Uint16:
		return func(str string) (any, error) {
			return parseUint(str, 16)
		}
	case reflect.Uint32:
		return func(str string) (any, error) {
			return parseUint(str, 32)
		}
	case reflect.Uint64:
		return func(str string) (any, error) {
			return parseUint(str, 64)
		}
	default:
		return func(str string) (any, error) {
			return nil, fmt.Errorf("unsupported str type %v", kind)
		}
	}
}
