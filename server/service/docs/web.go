package docs

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/route/cors"
	"github.com/TBXark/sphere/server/route/docs"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
)

type Target struct {
	Address string
	Spec    *swag.Spec
}

type Config struct {
	Address string
	Targets []Target
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

func (w *Web) Start(ctx context.Context) error {
	engine := gin.Default()
	cors.Setup(engine, []string{"*"})

	for _, spec := range w.config.Targets {
		if err := setup(spec.Spec, engine, spec.Address); err != nil {
			return err
		}
	}
	indexHTML := []byte(createIndex(w.config.Targets))
	engine.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", indexHTML)
	})

	w.server = &http.Server{
		Addr:    w.config.Address,
		Handler: engine.Handler(),
	}
	return ginx.Start(ctx, w.server, 30*time.Second)
}

func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}

func setup(spec *swag.Spec, router gin.IRouter, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}

	route := router.Group("/" + strings.ToLower(spec.InstanceName()))

	spec.Host = ""
	spec.BasePath = fmt.Sprintf("/%s/api", route.BasePath())
	if spec.Description == "" {
		spec.Description = fmt.Sprintf(" | proxy for %s", target)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	docs.Setup(route.Group("/doc"), spec)
	route.Any("/api/*path", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("path")
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

func createIndex(targets []Target) string {
	const indexHTML = `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="https://unpkg.com/purecss@3.0.0/build/pure-min.css">
    <style>
        .header {
            text-align: center;
            padding: 2em 0;
        }
        .content {
            max-width: 800px;
            margin: 0 auto;
        }
        .api-card {
            border: 1px solid #e1e1e1;
            padding: 1em;
            margin-bottom: 1em;
            border-radius: 4px;
        }
        .api-card h2 {
            margin-top: 0;
        }
        .api-card p {
            margin-bottom: 0.5em;
        }
        .footer {
            text-align: center;
            padding: 2em 0;
            color: #888;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>API Documentation Index</h1>
    </div>
    <div class="content">
        {{range .}}
        <div class="api-card pure-u-1 pure-u-md-1-2">
            <h2><a href="/{{.Spec.InstanceName | lower}}/doc/swagger/index.html">{{.Spec.InstanceName}}</a></h2>
            <p><strong>Description:</strong> {{.Spec.Description}}</p>
            <p><strong>Version:</strong> {{.Spec.Version}}</p>
			<p><strong>Proxy:</strong> {{.Address}}</p>
        </div>
        {{end}}
    </div>
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
