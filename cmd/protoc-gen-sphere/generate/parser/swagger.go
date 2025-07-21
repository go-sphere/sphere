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
	leading := string(m.Comments.Leading)
	if leading == "" {
		return ""
	}
	cmp := strings.Split(strings.TrimSuffix(leading, "\n"), "\n")
	if len(cmp) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("// %s %s", m.Desc.Name(), strings.TrimSpace(cmp[0])))
	if len(cmp) > 1 {
		for _, line := range cmp[1:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("// %s", strings.TrimSpace(line)))
		}
	}
	return strings.Join(lines, "\n")
}

type SwagParams struct {
	Method string
	Path   string
	Auth   string

	PathVars  []URIParamsField
	QueryVars []QueryFormField

	Body         string
	ResponseBody string

	DataResponse  string
	ErrorResponse string
}

var NoBodyMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
}

func BuildAnnotations(g *protogen.GeneratedFile, m *protogen.Method, config *SwagParams) (string, error) {
	var builder strings.Builder
	builder.WriteString("// @Summary " + string(m.Desc.Name()) + "\n")
	desc := strings.TrimSpace(string(m.Comments.Leading))
	if desc != "" {
		desc = strings.TrimSpace(strings.Join(strings.Split(desc, "\n"), ","))
		builder.WriteString("// @Description " + desc + "\n")
	}

	pkgName := string(m.Parent.Desc.ParentFile().Package())
	builder.WriteString("// @Tags " + strings.Join([]string{
		pkgName,
		pkgName + "." + string(m.Parent.Desc.Name()),
	}, ",") + "\n")

	builder.WriteString("// @Accept json\n")
	builder.WriteString("// @Produce json\n")

	// Add authentication if specified
	if config.Auth != "" {
		builder.WriteString(config.Auth + "\n")
	}

	// Add path parameters
	for _, param := range config.PathVars {
		paramType := buildSwaggerParamType(g, param.Field)
		builder.WriteString(fmt.Sprintf("// @Param %s path %s true \"%s\"\n", param.Name, paramType, param.Name))
	}
	// Add query parameters
	for _, param := range config.QueryVars {
		paramType := buildSwaggerParamType(g, param.Field)
		required := isFieldRequired(param.Field)
		builder.WriteString(fmt.Sprintf("// @Param %s query %s %v \"%s\"\n", param.Name, paramType, required, param.Name))
	}
	// Add a request body
	if _, ok := NoBodyMethods[config.Method]; !ok {
		bodyType, err := buildSwaggerParamTypeByPath(g, m, m.Input, config.Body)
		if err != nil {
			return "", err
		}
		builder.WriteString("// @Param request body " + bodyType + " true \"request body\"\n")
	}

	// Add a response body
	responseType, err := buildSwaggerParamTypeByPath(g, m, m.Output, config.ResponseBody)
	if err != nil {
		return "", err
	}
	builder.WriteString("// @Success 200 {object} " + config.DataResponse + "[" + responseType + "]\n")
	builder.WriteString("// @Failure 400,401,403,500,default {object} " + config.ErrorResponse + "\n")

	builder.WriteString("// @Router " + config.Path + " [" + strings.ToLower(config.Method) + "]\n")

	return builder.String(), nil
}

func buildSwaggerParamTypeByPath(g *protogen.GeneratedFile, m *protogen.Method, message *protogen.Message, path string) (string, error) {
	name := g.QualifiedGoIdent(message.GoIdent)
	if path != "" {
		field := FindProtoField(message, strings.Split(path, "."))
		if field == nil {
			return "", fmt.Errorf("method `%s.%s` field `%s` not found in message `%s`. File: `%s`",
				m.Parent.Desc.Name(),
				m.Desc.Name(),
				path,
				message.Desc.Name(),
				m.Parent.Location.SourceFile,
			)
		} else {
			name = buildSwaggerParamType(g, field)
		}
	}
	return name, nil
}

func buildSwaggerParamType(g *protogen.GeneratedFile, field *protogen.Field) string {
	switch {
	case field.Desc.IsMap():
		key := buildSingularSwaggerParamType(g, field.Message.Fields[0])
		val := buildSingularSwaggerParamType(g, field.Message.Fields[1])
		return fmt.Sprintf("map[%s]%s", key, val)
	case field.Desc.IsList():
		elemType := buildSingularSwaggerParamType(g, field)
		return fmt.Sprintf("[]%s", elemType)
	default:
		return buildSingularSwaggerParamType(g, field)
	}
}

func buildSingularSwaggerParamType(g *protogen.GeneratedFile, field *protogen.Field) string {
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
		if field.Enum != nil {
			return g.QualifiedGoIdent(field.Enum.GoIdent)
		}
		return "integer"
	case protoreflect.MessageKind:
		if field.Message != nil {
			return g.QualifiedGoIdent(field.Message.GoIdent)
		}
		return "any"
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
