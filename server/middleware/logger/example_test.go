package logger_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/stdx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/logger"
)

// printLogger is a log.BaseLogger that prints the level, message, and status
// attribute so the example output is deterministic. Real services pass a
// logger such as log.With(log.WithName("http")).
type printLogger struct{}

func (printLogger) Debug(msg string, attrs ...log.Attr) { printEntry("DEBUG", msg, attrs) }
func (printLogger) Info(msg string, attrs ...log.Attr)  { printEntry("INFO", msg, attrs) }
func (printLogger) Warn(msg string, attrs ...log.Attr)  { printEntry("WARN", msg, attrs) }
func (printLogger) Error(msg string, attrs ...log.Attr) { printEntry("ERROR", msg, attrs) }

func printEntry(level, msg string, attrs []log.Attr) {
	for _, attr := range attrs {
		if attr.Key == "status" {
			fmt.Println(level, msg, "status", attr.Value)
			return
		}
	}
	fmt.Println(level, msg)
}

func ExampleLog() {
	lg := printLogger{}
	engine := stdx.New()
	engine.Use(logger.Log(lg), logger.RecoveryLog(lg, false))
	root := engine.Group("")
	root.GET("/ok", func(ctx httpx.Context) error {
		return ctx.Text(http.StatusOK, "ok")
	})
	root.GET("/missing", func(ctx httpx.Context) error {
		return httpx.NewNotFoundError("not found")
	})
	root.GET("/panic", func(ctx httpx.Context) error {
		panic("boom")
	})

	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		fmt.Println("engine does not support in-process dispatch")
		return
	}
	for _, path := range []string{"/ok", "/missing", "/panic"} {
		resp, err := tr.Do(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			fmt.Println("request:", err)
			return
		}
		_ = resp.Body.Close()
		fmt.Println("client got", resp.StatusCode)
	}
	// Output:
	// INFO /ok status 200
	// client got 200
	// ERROR /missing status 404
	// client got 404
	// ERROR [Recovery from panic]
	// INFO /panic status 500
	// client got 500
}
