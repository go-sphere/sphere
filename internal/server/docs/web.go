package docs

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
	"github.com/tbxark/sphere/pkg/server/route/cors"
	"github.com/tbxark/sphere/pkg/server/route/docs"
	"github.com/tbxark/sphere/swagger/api"
	"github.com/tbxark/sphere/swagger/dash"
	"golang.org/x/exp/maps"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Targets struct {
	API  string `json:"api" yaml:"api"`
	Dash string `json:"dash" yaml:"dash"`
}

type Config struct {
	Address string  `json:"address" yaml:"address"`
	Targets Targets `json:"targets" yaml:"targets"`
}

type Web struct {
	config *Config
	server *http.Server
}

func NewWebServer(conf *Config) *Web {
	return &Web{
		config: conf,
	}
}

func (w *Web) Identifier() string {
	return "docs"
}

func (w *Web) Run() error {
	engine := gin.Default()
	cors.Setup(engine, []string{"*"})

	targets := map[string]*swag.Spec{
		w.config.Targets.API:  api.SwaggerInfoAPI,
		w.config.Targets.Dash: dash.SwaggerInfoDash,
	}
	for target, spec := range targets {
		if err := setup(spec, engine, target); err != nil {
			return err
		}
	}
	indexHTML := []byte(createIndex(maps.Values(targets)))
	engine.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", indexHTML)
	})

	w.server = &http.Server{
		Addr:    w.config.Address,
		Handler: engine.Handler(),
	}
	return w.server.ListenAndServe()
}

func (w *Web) Clean() error {
	if w.server != nil {
		return w.server.Close()
	}
	return nil
}

func setup(spec *swag.Spec, router gin.IRouter, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}

	route := router.Group("/" + strings.ToLower(spec.InstanceName()))

	spec.Host = ""
	spec.BasePath = fmt.Sprintf("/%s/api", route.BasePath())
	spec.Description = fmt.Sprintf("Proxy for %s", target)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	docs.Setup(route.Group("/doc"), spec)
	route.Any("/api/*path", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("path")
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

func createIndex(targets []*swag.Spec) string {
	const indexHTML = `<!DOCTYPE html>
<html>
<head>
	  <title>API Docs</title>
	  <meta charset="utf-8">
	  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
{{range .}}
	<h1><a href="/{{.InstanceName | lower}}/doc/swagger/index.html"> {{.InstanceName}} </a></h1>
	<p>{{.Description}}</p>
{{end}}
</body>
</html>
`
	tmpl := template.New("index")
	tmpl.Funcs(template.FuncMap{
		"lower": strings.ToLower,
	})
	tmpl, _ = tmpl.Parse(indexHTML)
	var sb strings.Builder
	_ = tmpl.Execute(&sb, targets)
	return sb.String()
}
