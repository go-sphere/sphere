package auth

import (
	"fmt"
	"reflect"
	"strconv"
)

func strParser(kind reflect.Kind) func(string) (any, error) {
	switch kind {
	case reflect.String:
		return func(str string) (any, error) {
			return str, nil
		}
	case reflect.Int:
		return func(str string) (any, error) {
			num, err := strconv.ParseInt(str, 10, strconv.IntSize)
			if err != nil {
				return nil, err
			}
			return int(num), nil
		}
	case reflect.Int8:
		return func(str string) (any, error) {
			num, err := strconv.ParseInt(str, 10, 8)
			if err != nil {
				return nil, err
			}
			return int8(num), nil
		}
	case reflect.Int16:
		return func(str string) (any, error) {
			num, err := strconv.ParseInt(str, 10, 16)
			if err != nil {
				return nil, err
			}
			return int16(num), nil
		}
	case reflect.Int32:
		return func(str string) (any, error) {
			num, err := strconv.ParseInt(str, 10, 32)
			if err != nil {
				return nil, err
			}
			return int32(num), nil
		}
	case reflect.Int64:
		return func(str string) (any, error) {
			num, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return nil, err
			}
			return num, nil
		}
	case reflect.Uint:
		return func(str string) (any, error) {
			num, err := strconv.ParseUint(str, 10, strconv.IntSize)
			if err != nil {
				return nil, err
			}
			return uint(num), nil
		}
	case reflect.Uint8:
		return func(str string) (any, error) {
			num, err := strconv.ParseUint(str, 10, 8)
			if err != nil {
				return nil, err
			}
			return uint8(num), nil
		}
	case reflect.Uint16:
		return func(str string) (any, error) {
			num, err := strconv.ParseUint(str, 10, 16)
			if err != nil {
				return nil, err
			}
			return uint16(num), nil
		}
	case reflect.Uint32:
		return func(str string) (any, error) {
			num, err := strconv.ParseUint(str, 10, 32)
			if err != nil {
				return nil, err
			}
			return uint32(num), nil
		}
	case reflect.Uint64:
		return func(str string) (any, error) {
			num, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				return nil, err
			}
			return num, nil
		}
	default:
		return func(str string) (any, error) {
			return nil, fmt.Errorf("unsupported str type %v", kind)
		}
	}
}
