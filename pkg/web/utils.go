package web

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"reflect"
	"strings"
)

func AddValue[T any](source *gin.H, keyPath string, builder func() (T, error)) {
	if *source == nil {
		*source = gin.H{}
	}
	value, err := builder()
	if err != nil {
		return
	}
	keys := strings.Split(keyPath, ".")
	current := *source
	for _, key := range keys[:len(keys)-1] {
		if _, ok := current[key]; !ok {
			current[key] = gin.H{}
		}
		current = current[key].(gin.H)
	}
	current[keys[len(keys)-1]] = value
}

func ConvertObjectToMap(obj interface{}) (gin.H, error) {
	if obj == nil {
		return gin.H{}, nil
	}
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return gin.H{}, nil
		}
		return ConvertObjectToMap(val.Elem().Interface())
	}
	if val.Kind() == reflect.Map {
		return DeepConvertObjectToMap(obj)
	}
	if val.Kind() != reflect.Struct {
		return nil, errors.New("invalid object type")
	}
	result := gin.H{}
	for i := 0; i < val.NumField(); i++ {
		tag := val.Type().Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		key := strings.Split(tag, ",")[0]
		result[key] = val.Field(i).Interface()
	}
	return result, nil
}

func DeepConvertObjectToMap(obj interface{}) (gin.H, error) {
	result := gin.H{}
	bytes, err := json.Marshal(obj)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(bytes, &result)
	return result, err
}

func ConvertArrayToSet[T comparable](source []T) []T {
	temp := make(map[T]any)
	for _, item := range source {
		temp[item] = struct{}{}
	}
	result := make([]T, 0, len(temp))
	for item := range temp {
		result = append(result, item)
	}
	return result
}

func ConvertArrayToMap[T comparable, V any](source []V, builder func(V) T) map[T]V {
	result := make(map[T]V, len(source))
	for _, item := range source {
		result[builder(item)] = item
	}
	return result
}
