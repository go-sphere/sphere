// Package middleware holds a benchmark over the real middleware stack a
// service registers, comparing httpx.Middleware (one adapter layer per
// middleware, chain driven by ctx.Next) with httpx.Interceptor (one chain
// composed into the route). Both run the same middlewares and produce the same
// response.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/ginx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/auth/authorizer"
	"github.com/go-sphere/sphere/server/middleware/auth"
	"github.com/go-sphere/sphere/server/middleware/logger"
)

type discardLogger struct{}

func (discardLogger) Debug(string, ...log.Attr) {}
func (discardLogger) Info(string, ...log.Attr)  {}
func (discardLogger) Warn(string, ...log.Attr)  {}
func (discardLogger) Error(string, ...log.Attr) {}

type benchClaims struct{}

func (benchClaims) GetUID() (int64, error)      { return 42, nil }
func (benchClaims) GetSubject() (string, error) { return "bench", nil }
func (benchClaims) GetRoles() ([]string, error) { return []string{"admin"}, nil }

type allowAll struct{}

func (allowAll) IsAllowed(string, string) bool { return true }

func benchParser() authorizer.Parser[int64, benchClaims] {
	return authorizer.ParserFunc[int64, benchClaims](
		func(context.Context, string) (benchClaims, error) { return benchClaims{}, nil },
	)
}

// A writer that keeps its header map but drops bodies, so the measurement is
// the chain and not the response buffer.
type discardWriter struct {
	header http.Header
	status int
}

func (w *discardWriter) Header() http.Header         { return w.header }
func (w *discardWriter) WriteHeader(status int)      { w.status = status }
func (w *discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// BenchmarkRealStack registers the shape sphere-layout uses: access log and
// panic recovery at the engine, authentication on one nested group, permission
// on the next, then the route.
func BenchmarkRealStack(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	lg := discardLogger{}
	parser := benchParser()
	acl := allowAll{}
	leaf := func(ctx httpx.Context) error { return ctx.NoContent(http.StatusNoContent) }

	// "none" is the same route with no middleware at all, so the work the
	// middlewares do themselves can be told apart from the cost of the form.
	for _, mode := range []string{"none", "middleware", "interceptor"} {
		b.Run(fmt.Sprintf("form=%s", mode), func(b *testing.B) {
			ge := gin.New()
			app := ginx.New(ginx.WithEngine(ge))
			switch mode {
			case "none":
				app.Group("/api").Group("/admin").GET("/route", leaf)
			case "middleware":
				app.Use(logger.Log(lg), logger.RecoveryLog(lg, true))
				authed := app.Group("/api", auth.NewAuthMiddleware(parser))
				admin := authed.Group("/admin", auth.NewPermissionMiddleware[int64]("bench", acl))
				admin.GET("/route", leaf)
			default:
				if !httpx.UseInterceptor(app, logger.LogInterceptor(lg), logger.RecoveryLogInterceptor(lg, true)) {
					b.Fatal("engine did not register interceptors natively")
				}
				authed := app.Group("/api")
				httpx.UseInterceptor(authed, auth.NewAuthInterceptor(parser))
				admin := authed.Group("/admin")
				httpx.UseInterceptor(admin, auth.NewPermissionInterceptor[int64]("bench", acl))
				admin.GET("/route", leaf)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/admin/route", nil)
			req.Header.Set(auth.AuthorizationHeader, "token")
			w := &discardWriter{header: make(http.Header)}
			ge.ServeHTTP(w, req)
			if w.status != http.StatusNoContent {
				b.Fatalf("status = %d, want %d", w.status, http.StatusNoContent)
			}

			b.ReportAllocs()
			for b.Loop() {
				clear(w.header)
				ge.ServeHTTP(w, req)
			}
		})
	}
}
