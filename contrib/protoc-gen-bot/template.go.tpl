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

type {{.ServiceType}}Codec[Update any, Message any] interface {
{{- range .MethodSets}}
    Decode{{.Name}}Request(ctx context.Context, update *Update) (*{{.Request}}, error)
    Encode{{.Name}}Response(ctx context.Context, reply *{{.Reply}}) (*Message, error)
{{- end}}
}

type {{.ServiceType}}MessageSender[Bot any, Update any, Message any] func(ctx context.Context, bot *Bot, update *Update, msg *Message) error

type {{.ServiceType}}Handler[Bot any, Update any, Message any] func(ctx context.Context, bot *Bot, update *Update) error

{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler[Bot any, Update any, Message any](srv {{$svrType}}Server, codec {{$svrType}}Codec[Update, Message], sender {{$svrType}}MessageSender[Bot, Update, Message]) {{$svrType}}Handler[Bot, Update, Message] {
    return func(ctx context.Context, bot *Bot, update *Update) error {
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
    		return sender(ctx, bot, update, msg)
    }
}
{{end}}

func Register{{.ServiceType}}BotServer[Bot any, Update any, Message any](srv {{.ServiceType}}Server, codec {{.ServiceType}}Codec[Update, Message], sender {{.ServiceType}}MessageSender[Bot, Update, Message]) map[string]{{.ServiceType}}Handler[Bot, Update, Message]{
	handlers := make(map[string]{{.ServiceType}}Handler[Bot, Update, Message])
{{- range .Methods}}
    handlers[BotHandler{{$svrType}}{{.OriginalName}}] = _{{$svrType}}_{{.Name}}{{.Num}}_Bot_Handler(srv, codec, sender)
{{- end}}
    return handlers
}
