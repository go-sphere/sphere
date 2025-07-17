package parser

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	validatepb "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func ConvertGinToSwaggerPath(ginPath string) string {
	//  :params -> {params}
	re := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	swaggerPath := re.ReplaceAllString(ginPath, "{$1}")
	//  *filepath -> {filepath}
	re2 := regexp.MustCompile(`\*([a-zA-Z_][a-zA-Z0-9_]*)`)
	swaggerPath = re2.ReplaceAllString(swaggerPath, "{$1}")
	return swaggerPath
}

func MethodCommend(m *protogen.Method) string {
	leading := m.Comments.Leading.String()
	if leading == "" {
		return ""
	}
	comment := strings.TrimSpace(strings.ReplaceAll(leading, "\n", " "))
	if comment != "" {
		comment = "// " + m.GoName + strings.TrimPrefix(strings.TrimSuffix(comment, "\n"), "//")
	}
	return comment
}

type SwagParams struct {
	Method string
	Path   string
	Auth   string

	PathVars  []URIParamsField
	QueryVars []QueryFormField

	DataResponse  string
	ErrorResponse string
}

var NoBodyMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
}

func BuildAnnotations(m *protogen.Method, config *SwagParams) string {
	var builder strings.Builder
	builder.WriteString("// @Summary " + string(m.Desc.Name()) + "\n")
	desc := MethodCommend(m)
	if desc != "" {
		desc = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(desc, "\n"), "//"))
		builder.WriteString("// @Description " + desc + "\n")
	}
	pkgName := string(m.Parent.Desc.ParentFile().Package())
	builder.WriteString("// @Tags " + strings.Join([]string{
		pkgName,
		pkgName + "." + string(m.Parent.Desc.Name()),
	}, ",") + "\n")
	builder.WriteString("// @Accept json\n")
	builder.WriteString("// @Produce json\n")
	if config.Auth != "" {
		builder.WriteString(config.Auth + "\n")
	}
	// Add path parameters
	for _, param := range config.PathVars {
		paramType := buildSwaggerParamType(param.Field)
		builder.WriteString(fmt.Sprintf("// @Param %s path %s true \"%s\"\n", param.Name, paramType, param.Name))
	}
	// Add query parameters
	for _, param := range config.QueryVars {
		paramType := buildSwaggerParamType(param.Field)
		required := isFieldRequired(param.Field)
		builder.WriteString(fmt.Sprintf("// @Param %s query %s %v \"%s\"\n", param.Name, paramType, required, param.Name))
	}
	// Add a request body
	if _, ok := NoBodyMethods[config.Method]; !ok {
		builder.WriteString("// @Param request body " + m.Input.GoIdent.GoName + " true \"request body\"\n")
	}
	builder.WriteString("// @Success 200 {object} " + config.DataResponse + "[" + m.Output.GoIdent.GoName + "]\n")
	builder.WriteString("// @Failure 400,401,403,500,default {object} " + config.ErrorResponse + "\n")
	builder.WriteString("// @Router " + config.Path + " [" + strings.ToLower(config.Method) + "]\n")
	return builder.String()
}

func buildSwaggerParamType(field *protogen.Field) string {
	switch {
	case field.Desc.IsMap():
		key := buildSingularSwaggerParamType(field.Message.Fields[0])
		val := buildSingularSwaggerParamType(field.Message.Fields[1])
		return fmt.Sprintf("map[%s]%s", key, val)
	case field.Desc.IsList():
		elemType := buildSingularSwaggerParamType(field)
		return fmt.Sprintf("[]%s", elemType)
	default:
		return buildSingularSwaggerParamType(field)
	}
}

func buildSingularSwaggerParamType(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Uint32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Fixed32Kind,
		protoreflect.Sfixed64Kind, protoreflect.Fixed64Kind:
		return "integer"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "string" // Swagger doesn't have a specific type for bytes, so we use string
	case protoreflect.EnumKind:
		return field.Enum.GoIdent.GoName
	case protoreflect.MessageKind:
		return field.Message.GoIdent.GoName
	default:
		return "any"
	}
}

func isFieldRequired(field *protogen.Field) bool {
	opts := field.Desc.Options()
	if opts == nil {
		return false
	}
	if proto.HasExtension(opts, validatepb.E_Field) {
		fieldConstraints := proto.GetExtension(opts, validatepb.E_Field).(*validatepb.FieldRules)
		if fieldConstraints != nil {
			return fieldConstraints.GetRequired()
		}
	}
	return false
}
