package ginx

import (
	"reflect"
	"strings"

	"github.com/fatih/structtag"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/form/v4"
)

var (
	urlBinding   = newUniverseDecoder("uri")
	queryBinding = newUniverseDecoder("form")
)

func newUniverseDecoder(key string) *form.Decoder {
	decoder := form.NewDecoder()
	decoder.SetTagName(key)
	decoder.RegisterTagNameFunc(func(field reflect.StructField) string {
		tags, err := structtag.Parse(string(field.Tag))
		if err != nil {
			return ""
		}
		if tag, e := tags.Get(key); e == nil {
			return tag.Name
		}
		if tag, e := tags.Get("protobuf"); e == nil {
			for _, option := range tag.Options {
				if strings.HasPrefix(option, "name=") {
					return strings.TrimPrefix(option, "name=")
				}
			}
		}
		return ""
	})
	return decoder
}

// ShouldBindUri binds URI path parameters to the given object using Gin's default binding.
// It delegates to gin.Context.ShouldBindUri for standard URI parameter binding.
func ShouldBindUri(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindUri(obj)
}

// ShouldBindQuery binds query parameters to the given object using Gin's default binding.
// It delegates to gin.Context.ShouldBindQuery for standard query parameter binding.
func ShouldBindQuery(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindQuery(obj)
}

// ShouldBindJSON binds JSON request body to the given object using Gin's default binding.
// It delegates to gin.Context.ShouldBindJSON for standard JSON binding.
func ShouldBindJSON(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindJSON(obj)
}

// ShouldBindHeader binds HTTP headers to the given object using Gin's default binding.
// It delegates to gin.Context.ShouldBindHeader for standard header binding.
func ShouldBindHeader(ctx *gin.Context, obj any) error {
	return ctx.ShouldBindHeader(obj)
}

// ShouldUniverseBindUri binds URI parameters using a custom form decoder that supports
// both standard struct tags and protobuf name resolution for field mapping.
func ShouldUniverseBindUri(ctx *gin.Context, obj any) error {
	m := make(map[string][]string, len(ctx.Params))
	for _, v := range ctx.Params {
		m[v.Key] = []string{v.Value}
	}
	return urlBinding.Decode(obj, m)
}

// ShouldUniverseBindQuery binds query parameters using a custom form decoder that supports
// both standard struct tags and protobuf name resolution for field mapping.
func ShouldUniverseBindQuery(ctx *gin.Context, obj any) error {
	return queryBinding.Decode(obj, ctx.Request.URL.Query())
}

// ShouldUniverseBind performs comprehensive binding from multiple HTTP request sources.
// It can bind from JSON body, query parameters, and URI parameters based on the boolean flags.
// The binding order is: body -> query -> uri, with each step potentially overriding previous values.
func ShouldUniverseBind(ctx *gin.Context, obj any, uri, query, body bool) error {
	if body {
		if err := ShouldBindJSON(ctx, obj); err != nil {
			return err
		}
	}
	if query {
		if err := ShouldUniverseBindQuery(ctx, obj); err != nil {
			return err
		}
	}
	if uri {
		if err := ShouldUniverseBindUri(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
