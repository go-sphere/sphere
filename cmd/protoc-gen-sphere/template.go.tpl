{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}


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
func _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv {{$svrType}}HTTPServer) func(ctx *gin.Context)  {
	return ginx.WithJson(func(ctx *gin.Context) (*{{.Reply}}, error) {
		var in {{.Request}}
		{{- if .HasBody}}
		if err := ctx.ShouldBindJSON(&in{{.Body}}); err != nil {
			return nil, err
		}
		{{- end}}
		{{- if .HasQuery}}
		if err := ginx.ShouldBindQuery(ctx, &in); err != nil {
			return nil, err
		}
		{{- end}}
		{{- if .HasVars}}
		if err := ginx.ShouldBindUri(ctx, &in); err != nil {
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

func Register{{.ServiceType}}HTTPServer(route gin.IRouter, srv {{.ServiceType}}HTTPServer) {
	r := route.Group("/")
	{{- range .Methods}}
	r.{{.Method}}("{{.GinPath}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end}}
}