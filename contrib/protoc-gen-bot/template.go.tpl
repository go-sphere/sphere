{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$clientType := .ClientType}}
{{$updateType := .UpdateType}}
{{$messageType := .MessageType}}

{{- range .MethodSets}}
const BotHandler{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}


type {{.ServiceType}}Server interface {
{{- range .MethodSets}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type {{.ServiceType}}Codec interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, update *{{$updateType}}) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*{{$messageType}}, error)
{{- end}}
}

type {{.ServiceType}}Handler func(ctx context.Context,  client *{{.ClientType}}, update *{{.UpdateType}}) error

type {{.ServiceType}}MessageSender func(ctx context.Context, client *{{.ClientType}}, update *{{.UpdateType}}, msg *{{.MessageType}}) error

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv {{$svrType}}Server, codec {{$svrType}}Codec, sender {{$svrType}}MessageSender) {{$svrType}}Handler {
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

func Register{{.ServiceType}}BotServer(srv {{.ServiceType}}Server, codec {{.ServiceType}}Codec, sender {{.ServiceType}}MessageSender) map[string]{{.ServiceType}}Handler {
	handlers := make(map[string]{{.ServiceType}}Handler)
{{- range .Methods}}
    handlers[BotHandler{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv, codec, sender)
{{- end}}
    return handlers
}
