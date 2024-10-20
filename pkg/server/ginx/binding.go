package ginx

import (
	"errors"
	"github.com/gin-gonic/gin/binding"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type TagNameGetter interface {
	Get(f reflect.StructField) string
}

type TagNameFunc func(f reflect.StructField) string

func (f TagNameFunc) Get(field reflect.StructField) string {
	return f(field)
}

func simpleTagNameFunc(tag string) TagNameFunc {
	// example: `form:"name,required,min=1,max=100"`
	return func(f reflect.StructField) string {
		return strings.Split(f.Tag.Get(tag), ",")[0]
	}
}

func protobufTagNameFunc(f reflect.StructField) string {
	// example: `protobuf:"bytes,1,opt,name=code,proto3"`
	cmp := strings.Split(f.Tag.Get("protobuf"), ",")
	for _, s := range cmp {
		if strings.HasPrefix(s, "name=") {
			return strings.TrimPrefix(s, "name=")
		}
	}
	return ""
}

func multiTagNameFunc(fns ...TagNameFunc) TagNameFunc {
	return func(f reflect.StructField) string {
		for _, fn := range fns {
			if name := fn.Get(f); name != "" && name != "-" {
				return name
			}
		}
		return ""
	}
}

type UniverseBinding struct {
	tagName     string
	tagGetter   TagNameGetter
	valueGetter func(c *gin.Context, name string) (string, bool)
}

func (u *UniverseBinding) Name() string {
	return "universe"
}

var (
	ErrBindingElementMustBePointer = errors.New("binding element must be a pointer")
	ErrBindingElementMustBeStruct  = errors.New("binding element must be a struct")
	ErrUnsupportedFieldType        = errors.New("unsupported field type")
)

func (u *UniverseBinding) Bind(c *gin.Context, obj interface{}) error {
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return ErrBindingElementMustBePointer
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return ErrBindingElementMustBeStruct
	}

	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		fieldValue := value.Field(i)

		tag := u.tagGetter.Get(field)
		if tag == "" || tag == "-" {
			continue
		}
		val, ok := u.valueGetter(c, tag)
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
			return ErrUnsupportedFieldType
		}
	}
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}

var (
	uriBinding = &UniverseBinding{
		tagName:   "uri",
		tagGetter: multiTagNameFunc(simpleTagNameFunc("uri"), protobufTagNameFunc),
		valueGetter: func(c *gin.Context, name string) (string, bool) {
			return c.Params.Get(name)
		},
	}
	queryBinding = &UniverseBinding{
		tagName:   "form",
		tagGetter: multiTagNameFunc(simpleTagNameFunc("form"), protobufTagNameFunc),
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
