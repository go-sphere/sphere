package ginx

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	ErrBindingElementMustBePointer = errors.New("universe binding element must be a pointer")
	ErrBindingElementMustBeStruct  = errors.New("universe binding element must be a struct")
	ErrUnsupportedFieldType        = errors.New("unsupported field type")
)

var invalidTags = map[string]bool{
	"":  true,
	"-": true,
}

type ValueGetterFunc = func(ctx *gin.Context, name string) (string, bool)

type TagNameGetter interface {
	Get(f reflect.StructField) (string, bool)
}

type TagNameFunc func(f reflect.StructField) (string, bool)

func (f TagNameFunc) Get(field reflect.StructField) (string, bool) {
	return f(field)
}

func simpleTagNameFunc(tag string) TagNameFunc {
	return func(f reflect.StructField) (string, bool) {
		cmp := strings.Split(f.Tag.Get(tag), ",")
		if len(cmp) == 0 {
			return "", false
		}
		return cmp[0], true
	}
}

func protobufTagNameFunc(f reflect.StructField) (string, bool) {
	cmp := strings.Split(f.Tag.Get("protobuf"), ",")
	for _, s := range cmp {
		if strings.HasPrefix(s, "name=") {
			return strings.TrimPrefix(s, "name="), true
		}
	}
	return "", false
}

func multiTagNameFunc(fns ...TagNameFunc) TagNameFunc {
	return func(f reflect.StructField) (string, bool) {
		for _, fn := range fns {
			name, ok := fn.Get(f)
			if ok && !invalidTags[name] {
				return name, true
			}
		}
		return "", false
	}
}

type fieldInfo struct {
	index []int
	tag   string
}

type UniverseBinding struct {
	tagName     string
	tagGetter   TagNameGetter
	valueGetter ValueGetterFunc

	cache      sync.RWMutex
	fieldCache map[reflect.Type][]*fieldInfo
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
		fieldValue := value.FieldByIndex(field.index)
		if !fieldValue.CanSet() {
			continue
		}
		if err := u.setFieldValue(fieldValue, val); err != nil {
			return err
		}
	}
	return nil
}

func (u *UniverseBinding) getFieldInfo(typ reflect.Type) []*fieldInfo {
	u.cache.RLock()
	if fields, ok := u.fieldCache[typ]; ok {
		u.cache.RUnlock()
		return fields
	}
	u.cache.RUnlock()

	u.cache.Lock()
	defer u.cache.Unlock()
	if fields, ok := u.fieldCache[typ]; ok {
		return fields
	}
	fields := u.analyzeFields(typ)
	u.fieldCache[typ] = fields
	return fields
}

func (u *UniverseBinding) analyzeFields(typ reflect.Type) []*fieldInfo {
	return u.recursiveAnalyze(typ, nil)
}

func (u *UniverseBinding) recursiveAnalyze(typ reflect.Type, parentIndex []int) []*fieldInfo {
	var fields []*fieldInfo
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		currentIndex := make([]int, len(parentIndex)+1)
		copy(currentIndex, parentIndex)
		currentIndex[len(parentIndex)] = i

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			fields = append(fields, u.recursiveAnalyze(field.Type, currentIndex)...)
			continue
		}

		tag, ok := u.tagGetter.Get(field)
		if !ok || invalidTags[tag] {
			continue
		}
		info := &fieldInfo{
			index: currentIndex,
			tag:   tag,
		}
		fields = append(fields, info)
	}
	return fields
}

func (u *UniverseBinding) setFieldValue(fieldValue reflect.Value, val string) error {
	kind := fieldValue.Kind()
	if kind == reflect.Ptr {
		if fieldValue.IsNil() {
			newVal := reflect.New(fieldValue.Type().Elem())
			fieldValue.Set(newVal)
		}
		return u.setFieldValue(fieldValue.Elem(), val)
	}
	switch kind {
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

var (
	uriBinding = &UniverseBinding{
		tagName:   "uri",
		tagGetter: multiTagNameFunc(simpleTagNameFunc("uri"), protobufTagNameFunc),
		valueGetter: func(ctx *gin.Context, name string) (string, bool) {
			return ctx.Params.Get(name)
		},
		fieldCache: make(map[reflect.Type][]*fieldInfo, 32),
	}
	queryBinding = &UniverseBinding{
		tagName:   "form",
		tagGetter: multiTagNameFunc(simpleTagNameFunc("form"), protobufTagNameFunc),
		valueGetter: func(ctx *gin.Context, name string) (string, bool) {
			return ctx.GetQuery(name)
		},
		fieldCache: make(map[reflect.Type][]*fieldInfo, 32),
	}
)

func ShouldUniverseBindUri(ctx *gin.Context, obj any) error {
	return uriBinding.Bind(ctx, obj)
}

func ShouldUniverseBindQuery(ctx *gin.Context, obj any) error {
	return queryBinding.Bind(ctx, obj)
}

func ShouldBindQuery(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindQuery(obj)
}

func ShouldBindUri(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindUri(obj)
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
