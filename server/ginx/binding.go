package ginx

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Errors

var (
	ErrBindingElementMustBePointer = errors.New("universe binding element must be a pointer")
	ErrBindingElementMustBeStruct  = errors.New("universe binding element must be a struct")
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

type fieldInfo struct {
	index int
	tag   string
}
type UniverseBinding struct {
	tagName     string
	tagGetter   TagNameGetter
	valueGetter func(ctx *gin.Context, name string) (string, bool)
	cache       sync.Map
}

func (u *UniverseBinding) Name() string {
	return "universe"
}

func (u *UniverseBinding) Bind(ctx *gin.Context, obj any) error {
	value := reflect.ValueOf(obj)
	if value.Kind() != reflect.Ptr {
		return ErrBindingElementMustBePointer
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return ErrBindingElementMustBeStruct
	}

	typ := value.Type()
	fields := u.getFieldInfo(typ)

	for _, field := range fields {
		val, exist := u.valueGetter(ctx, field.tag)
		if !exist {
			continue
		}
		fieldValue := value.Field(field.index)
		if err := u.setFieldValue(fieldValue, val); err != nil {
			return err
		}
	}
	return nil
}

func (u *UniverseBinding) getFieldInfo(typ reflect.Type) []fieldInfo {
	if cached, ok := u.cache.Load(typ); ok {
		return cached.([]fieldInfo)
	}
	fields := u.analyzeFields(typ)
	u.cache.Store(typ, fields)
	return fields
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
	if fieldValue.Kind() == reflect.Ptr {
		if fieldValue.IsNil() {
			newVal := reflect.New(fieldValue.Type().Elem())
			fieldValue.Set(newVal)
		}
		return u.setFieldValue(fieldValue.Elem(), val)
	}

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
		valueGetter: func(ctx *gin.Context, name string) (string, bool) {
			return ctx.Params.Get(name)
		},
	}
	queryBinding = &UniverseBinding{
		tagName:   "form",
		tagGetter: multiTagNameFunc(simpleTagNameFunc("form"), protobufTagNameFunc),
		valueGetter: func(ctx *gin.Context, name string) (string, bool) {
			return ctx.GetQuery(name)
		},
	}
)

/// Public functions

func ShouldBindUri(ctx *gin.Context, obj any) error {
	return uriBinding.Bind(ctx, obj)
}

func ShouldBindQuery(ctx *gin.Context, obj any) error {
	return queryBinding.Bind(ctx, obj)
}

func ShouldBindJSON(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindJSON(obj)
}

func ShouldBind(ctx *gin.Context, obj any, uri, query, body bool) error {
	if body {
		if err := ShouldBindJSON(ctx, obj); err != nil {
			return err
		}
	}
	if uri {
		if err := ShouldBindUri(ctx, obj); err != nil {
			return err
		}
	}
	if query {
		if err := ShouldBindQuery(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
