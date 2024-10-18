package ginx

import (
	"errors"
	"github.com/gin-gonic/gin/binding"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type UniverseBinding struct {
	tagName     string
	valueGetter func(c *gin.Context, name string) (string, bool)
}

func (u *UniverseBinding) Name() string {
	return "custom"
}

func (u *UniverseBinding) Bind(c *gin.Context, obj interface{}) error {
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return errors.New("binding element must be a pointer")
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return errors.New("binding element must be a struct")
	}

	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		fieldValue := value.Field(i)

		tag := field.Tag.Get(u.tagName)
		if tag == "" {
			continue
		}

		// 尝试获取值
		bindName := strings.Split(tag, ",")[0]
		val, ok := u.valueGetter(c, bindName)
		if !ok {
			continue
		}

		// 根据字段类型进行转换
		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(val)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
			fieldValue.SetInt(intVal)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			uintVal, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return err
			}
			fieldValue.SetUint(uintVal)
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return err
			}
			fieldValue.SetFloat(floatVal)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			fieldValue.SetBool(boolVal)
		default:
			return errors.New("unsupported field type")
		}
	}
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}

var (
	uriBinding = &UniverseBinding{
		tagName: "uri",
		valueGetter: func(c *gin.Context, name string) (string, bool) {
			return c.Params.Get(name)
		},
	}
	queryBinding = &UniverseBinding{
		tagName: "form",
		valueGetter: func(c *gin.Context, name string) (string, bool) {
			return c.GetQuery(name)
		},
	}
)

func ShouldBindUri(c *gin.Context, obj interface{}) error {
	return uriBinding.Bind(c, obj)
}

func ShouldBindQuery(c *gin.Context, obj interface{}) error {
	return queryBinding.Bind(c, obj)
}

func ShouldBindJSON(c *gin.Context, obj interface{}) error {
	return c.ShouldBindJSON(obj)
}
