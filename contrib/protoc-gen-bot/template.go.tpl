{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$requestType := .RequestType}}
{{$responseType := .ResponseType}}
{{$extraDataType := .ExtraDataType}}
{{$newExtraDataFunc := .NewExtraDataFunc}}

{{- range .MethodSets}}
const OperationBot{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

{{- range .MethodSets}}
    {{- if .Extra}}
var ExtraData{{$svrType}}{{.Name}} = {{$newExtraDataFunc}}(map[string]string{
    {{- range $key, $value := .Extra}}
    "{{$key}}": "{{$value}}",
    {{- end}}
})
    {{- end}}
{{- end}}

func GetExtraDataByBot{{$svrType}}Operation(operation string) *{{.ExtraDataType}} {
    switch operation {
    {{- range .MethodSets}}
    {{- if .Extra}}
    case OperationBot{{$svrType}}{{.OriginalName}}:
        return &ExtraData{{$svrType}}{{.Name}}
    {{- end}}
    {{- end}}
    default:
        return nil
    }
}

type {{.ServiceType}}BotServer interface {
{{- range .MethodSets}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type {{.ServiceType}}BotCodec interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, update *{{$requestType}}) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*{{$responseType}}, error)
{{- end}}
}

type {{.ServiceType}}BotHandler func(ctx context.Context, request *{{.RequestType}}) error

type {{.ServiceType}}BotSender func(ctx context.Context, request *{{.RequestType}}, msg *{{.ResponseType}}) error

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv {{$svrType}}BotServer, codec {{$svrType}}BotCodec, sender {{$svrType}}BotSender) {{$svrType}}BotHandler {
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

func Register{{.ServiceType}}BotServer(srv {{.ServiceType}}BotServer, codec {{.ServiceType}}BotCodec, sender {{.ServiceType}}BotSender) map[string]{{.ServiceType}}BotHandler {
	handlers := make(map[string]{{.ServiceType}}BotHandler)
{{- range .Methods}}
    handlers[OperationBot{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv, codec, sender)
{{- end}}
    return handlers
}
