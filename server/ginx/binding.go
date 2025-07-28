package ginx

import (
	"reflect"
	"strings"

	"github.com/fatih/structtag"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/form/v4"
)

var (
	urlBinding   = newDecoder("uri")
	queryBinding = newDecoder("form")
)

func newDecoder(key string) *form.Decoder {
	decoder := form.NewDecoder()
	decoder.SetTagName(key)
	decoder.RegisterTagNameFunc(tagNameFunc(key))
	return decoder
}

func tagNameFunc(key string) func(field reflect.StructField) string {
	return func(field reflect.StructField) string {
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
	}
}

func ShouldUniverseBindUri(ctx *gin.Context, obj any) error {
	m := make(map[string][]string, len(ctx.Params))
	for _, v := range ctx.Params {
		m[v.Key] = []string{v.Value}
	}
	return urlBinding.Decode(obj, m)
}

func ShouldUniverseBindQuery(ctx *gin.Context, obj any) error {
	return queryBinding.Decode(obj, ctx.Request.URL.Query())
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
