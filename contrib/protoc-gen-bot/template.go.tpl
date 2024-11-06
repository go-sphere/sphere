{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}

{{- range .MethodSets}}
const BotHandler{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}


type {{.ServiceType}}Server interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

type {{.ServiceType}}Codec interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, update *models.Update) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*telegram.Message, error)
{{- end}}
}


{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv {{$svrType}}Server, codec {{$svrType}}Codec) telegram.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) error {
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
    		return telegram.SendMessage(ctx, b, update, msg)
    }
}
{{end}}

func Register{{.ServiceType}}BotServer(srv {{$svrType}}Server, codec {{.ServiceType}}Codec) map[string]telegram.HandlerFunc {
	handlers := make(map[string]telegram.HandlerFunc)
{{- range .Methods}}
    handlers[BotHandler{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv, codec)
{{- end}}
    return handlers
}
