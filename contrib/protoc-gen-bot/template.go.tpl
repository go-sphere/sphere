{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$clientType := .ClientType}}
{{$updateType := .UpdateType}}
{{$messageType := .MessageType}}
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
    Decode{{.Name}}Request(ctx context.Context, update *{{$updateType}}) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*{{$messageType}}, error)
{{- end}}
}

type {{.ServiceType}}BotHandler func(ctx context.Context,  client *{{.ClientType}}, update *{{.UpdateType}}) error

type {{.ServiceType}}BotSender func(ctx context.Context, client *{{.ClientType}}, update *{{.UpdateType}}, msg *{{.MessageType}}) error

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv {{$svrType}}BotServer, codec {{$svrType}}BotCodec, sender {{$svrType}}BotSender) {{$svrType}}BotHandler {
    return func(ctx context.Context, client *{{$clientType}}, update *{{$updateType}}) error {
    		req, err := codec.Decode{{.Name}}Request(ctx, update)
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
    		return sender(ctx, client, update, msg)
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
