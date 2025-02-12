package ginx

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// Errors

var (
	ErrBindingElementMustBePointer = errors.New("binding element must be a pointer")
	ErrBindingElementMustBeStruct  = errors.New("binding element must be a struct")
	ErrUnsupportedFieldType        = errors.New("unsupported field type")
)

// TagNameFunc

type TagNameGetter interface {
	Get(f reflect.StructField) string
}

type TagNameFunc func(f reflect.StructField) string

func (f TagNameFunc) Get(field reflect.StructField) string {
	return f(field)
}

func simpleTagNameFunc(tag string) TagNameFunc {
	return func(f reflect.StructField) string {
		return strings.Split(f.Tag.Get(tag), ",")[0]
	}
}

func protobufTagNameFunc(f reflect.StructField) string {
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

// UniverseBinding

type UniverseBinding struct {
	tagName     string
	tagGetter   TagNameGetter
	valueGetter func(c *gin.Context, name string) (string, bool)
	cache       sync.Map
}

func (u *UniverseBinding) Name() string {
	return "universe"
}

func (u *UniverseBinding) Bind(c *gin.Context, obj interface{}) error {
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return ErrBindingElementMustBePointer
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return ErrBindingElementMustBeStruct
	}

	typ := value.Type()
	fields, ok := u.getFieldsFromCache(typ)
	if !ok {
		fields = u.analyzeFields(typ)
		u.cache.Store(typ, fields)
	}

	for _, field := range fields {
		val, exist := u.valueGetter(c, field.tag)
		if !exist {
			continue
		}
		fieldValue := value.Field(field.index)
		if err := u.setFieldValue(fieldValue, val); err != nil {
			return err
		}
	}
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}

/// fieldInfo

type fieldInfo struct {
	index int
	tag   string
}

func (u *UniverseBinding) getFieldsFromCache(typ reflect.Type) ([]fieldInfo, bool) {
	if cached, ok := u.cache.Load(typ); ok {
		return cached.([]fieldInfo), true
	}
	return nil, false
}

func (u *UniverseBinding) analyzeFields(typ reflect.Type) []fieldInfo {
	var fields []fieldInfo
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := u.tagGetter.Get(field)
		if tag == "" || tag == "-" {
			continue
		}
		fields = append(fields, fieldInfo{index: i, tag: tag})
	}
	return fields
}

func (u *UniverseBinding) setFieldValue(fieldValue reflect.Value, val string) error {
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
	return nil
}

/// Predefined bindings

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

/// Public functions

func ShouldBindUri(c *gin.Context, obj interface{}) error {
	return uriBinding.Bind(c, obj)
}

func ShouldBindQuery(c *gin.Context, obj interface{}) error {
	return queryBinding.Bind(c, obj)
}

func ShouldBindJSON(c *gin.Context, obj interface{}) error {
	return c.ShouldBindJSON(obj)
}

func ShouldBind(ctx *gin.Context, obj interface{}, uri, query, body bool) error {
	if body {
		if err := ShouldBindJSON(ctx, obj); err != nil {
			return err
		}
	}
	if query {
		if err := ShouldBindQuery(ctx, obj); err != nil {
			return err
		}
	}
	if uri {
		if err := ShouldBindUri(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
