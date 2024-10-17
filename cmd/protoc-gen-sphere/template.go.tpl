{{$svrType := .ServiceType}}
{{$svrName := .ServiceName}}


type {{.ServiceType}}HTTPServer interface {
{{- range .MethodSets}}
	{{- if ne .Comment ""}}
	{{.Comment}}
	{{- end}}
	{{.Name}}(*gin.Context, *{{.Request}}) (*{{.Reply}}, error)
{{- end}}
}



{{range .Methods}}
func _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv {{$svrType}}HTTPServer) func(ctx *gin.Context)  {
	return ginx.WithJson(func(ctx *gin.Context) (*{{.Reply}}, error) {
		var in {{.Request}}
		{{- if .HasBody}}
		if err := ctx.ShouldBindJSON(&in{{.Body}}); err != nil {
			return nil, err
		}
		{{- end}}
		if err := ctx.ShouldBindQuery(&in); err != nil {
			return nil, err
		}
		{{- if .HasVars}}
		if err := ctx.ShouldBindUri(&in); err != nil {
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
	r.{{.Method}}("{{.Path}}", _{{$svrType}}_{{.Name}}{{.Num}}_HTTP_Handler(srv))
	{{- end}}
}