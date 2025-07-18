{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$optionsKey := .OptionsKey}}
{{$requestType := .Package.RequestType}}
{{$responseType := .Package.ResponseType}}
{{$extraDataType := .Package.ExtraDataType}}
{{$newExtraDataFunc := .Package.NewExtraDataFunc}}

{{$handlerType := printf "func(ctx context.Context, request *%s) error" $requestType}}
{{$renderType := printf "func(ctx context.Context, request *%s, msg *%s) error" $requestType $responseType}}

{{- range .MethodSets}}
const Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

{{- if ne $extraDataType ""}}
{{- range .MethodSets}}
    {{- if .Extra}}
var Extra{{$optionsKey}}Data{{$svrType}}{{.Name}} = {{$newExtraDataFunc}}(map[string]string{
    {{- range $key, $value := .Extra}}
    "{{$key}}": "{{$value}}",
    {{- end}}
})
    {{- end}}
{{- end}}
{{- end}}

func GetExtra{{$optionsKey}}DataBy{{$svrType}}Operation(operation string) *{{$extraDataType}} {
    switch operation {
    {{- range .MethodSets}}
    {{- if .Extra}}
    case Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}}:
        return Extra{{$optionsKey}}Data{{$svrType}}{{.Name}}
    {{- end}}
    {{- end}}
    default:
        return nil
    }
}

func GetAll{{$optionsKey}}{{$svrType}}Operations() []string {
    return []string{
    {{- range .MethodSets}}
    Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}},
    {{- end}}
    }
}

type {{.ServiceType}}{{$optionsKey}}Server interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type {{.ServiceType}}{{$optionsKey}}Codec interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, request *{{$requestType}}) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, response *{{.Reply}}) (*{{$responseType}}, error)
{{- end}}
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_{{$optionsKey}}_Handler(srv {{$svrType}}{{$optionsKey}}Server, codec {{$svrType}}{{$optionsKey}}Codec, render {{$renderType}}) {{$handlerType}} {
    return func(ctx context.Context, request *{{$requestType}}) error {
    		req, err := codec.Decode{{.Name}}Request(ctx, request)
    		if err != nil {
    			return err
    		}
    		resp, err := srv.{{.Name}}(ctx, req)
    		if err != nil {
    			return err
    		}
    		msg, err := codec.Encode{{.Name}}Response(ctx, resp)
    		if err != nil {
    			return err
    		}
    		return render(ctx, request, msg)
    }
}
{{end}}

func Register{{.ServiceType}}{{$optionsKey}}Server(srv {{.ServiceType}}{{$optionsKey}}Server, codec {{.ServiceType}}{{$optionsKey}}Codec, render {{$renderType}}) map[string]{{$handlerType}} {
	handlers := make(map[string]{{$handlerType}})
{{- range .Methods}}
    handlers[Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_{{$optionsKey}}_Handler(srv, codec, render)
{{- end}}
    return handlers
}
