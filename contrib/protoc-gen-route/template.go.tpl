{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$optionsKey := .OptionsKey}}
{{$requestType := .RequestType}}
{{$responseType := .ResponseType}}
{{$extraDataType := .ExtraDataType}}
{{$newExtraDataFunc := .NewExtraDataFunc}}

{{$handlerType := printf "func(ctx context.Context, request *%s) error" $requestType}}
{{$senderType := printf "func(ctx context.Context, request *%s, msg *%s) error" $requestType $responseType}}

{{- range .MethodSets}}
const Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

{{- if ne .ExtraDataType ""}}
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

func GetExtra{{$optionsKey}}DataBy{{$svrType}}Operation(operation string) *{{.ExtraDataType}} {
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
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type {{.ServiceType}}{{$optionsKey}}Codec interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, update *{{$requestType}}) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*{{$responseType}}, error)
{{- end}}
}

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_{{$optionsKey}}_Handler(srv {{$svrType}}{{$optionsKey}}Server, codec {{$svrType}}{{$optionsKey}}Codec, sender {{$senderType}}) {{$handlerType}} {
    return func(ctx context.Context, request *{{$requestType}}) error {
    		req, err := codec.Decode{{.Name}}Request(ctx, request)
    		if err != nil {
    			return err
    		}
    		reply, err := srv.{{.Name}}(ctx, req)
    		if err != nil {
    			return err
    		}
    		msg, err := codec.Encode{{.Name}}Response(ctx, reply)
    		if err != nil {
    			return err
    		}
    		return sender(ctx, request, msg)
    }
}
{{end}}

func Register{{.ServiceType}}{{$optionsKey}}Server(srv {{.ServiceType}}{{$optionsKey}}Server, codec {{.ServiceType}}{{$optionsKey}}Codec, sender {{$senderType}}) map[string]{{$handlerType}} {
	handlers := make(map[string]{{$handlerType}})
{{- range .Methods}}
    handlers[Operation{{$optionsKey}}{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_{{$optionsKey}}_Handler(srv, codec, sender)
{{- end}}
    return handlers
}
