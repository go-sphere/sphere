{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}
{{$packageDesc := .Package}}

{{- range .MethodSets}}
const Operation{{$svrType}}{{.OriginalName}} = "/{{$svrName}}/{{.OriginalName}}"
{{- end}}

var Endpoints{{.ServiceType}} = [...][3]string{
	{{- range .Methods}}
	{Operation{{$svrType}}{{.OriginalName}}, "{{.Method}}", "{{.GinPath}}" },
	{{- end}}
}

type {{.ServiceType}}HTTPServer interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(context.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}

{{range .Methods}}
	{{- if ne .Swagger ""}}
	{{.Swagger}}
	{{- end -}}
func _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv {{$svrType}}HTTPServer) func(ctx *{{$packageDesc.ContextType}})  {
	return {{$packageDesc.ServerHandlerWrapperFunc}}(func(ctx *{{$packageDesc.ContextType}}) (*{{.Reply}}, error) {
		var in {{.Request}}
		{{- if .HasBody}}
		if err := {{$packageDesc.ParseJsonFunc}}(ctx, &in{{.Body}}); err != nil {
			return nil, err
		}
		{{- end}}
		{{- if .HasQuery}}
		if err := {{$packageDesc.ParseFormFunc}}(ctx, &in); err != nil {
			return nil, err
		}
		{{- end}}
		{{- if .HasVars}}
		if err := {{$packageDesc.ParseUriFunc}}(ctx, &in); err != nil {
			return nil, err
		}
		{{- end}}
		{{- if .NeedValidate}}
		if err := {{$packageDesc.ValidateFunc}}(&in); err != nil {
            return nil, err
        }
        {{- end}}
		out, err := srv.{{.Name}}(ctx, &in)
		if err != nil {
			return nil, err
		}
		return out, nil
	})
}
{{end}}

func Register{{.ServiceType}}HTTPServer(route {{.Package.RouterType}}, srv {{.ServiceType}}HTTPServer) {
	r := route.Group("/")
	{{- range .Methods}}
	r.{{.Method}}("{{.GinPath}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end}}
}
