package httpz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/httpxmock"
	"github.com/go-sphere/sphere/log"
)

// ctxCapture records the context each entry was logged with.
type ctxCapture struct {
	mu   sync.Mutex
	ctxs map[string]context.Context
}

func (c *ctxCapture) Log(ctx context.Context, _ log.Level, msg string, _ ...log.Attr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctxs[msg] = ctx
}
func (c *ctxCapture) Sync() error                    { return nil }
func (c *ctxCapture) With(...log.Option) log.Backend { return c }

func (c *ctxCapture) requestID(msg string) (string, bool) {
	c.mu.Lock()
	ctx, ok := c.ctxs[msg]
	c.mu.Unlock()
	if !ok {
		return "", false
	}
	for _, a := range log.AttrsFromContext(ctx) {
		if a.Key == "request_id" {
			return a.Value.String(), true
		}
	}
	return "", true
}

// TestPanicLogsCarryRequestContext pins that the panic logs of WithRecover and
// the WithSSE producer are written with the request context, so attrs attached
// by ContextWithAttrs (request_id, trace_id) reach the log backend.
func TestPanicLogsCarryRequestContext(t *testing.T) {
	capture := &ctxCapture{ctxs: map[string]context.Context{}}
	log.InitWithBackends(capture)
	t.Cleanup(func() { log.InitWithBackends(&log.StdioBackend{}) })

	reqCtx := log.ContextWithAttrs(t.Context(), log.String("request_id", "req-1"))

	t.Run("WithRecover", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx)
		ctx := httpxmock.New(req)
		handler := WithRecover("recover boom", func(httpx.Context) error { panic("boom") })
		if err := handler(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		got, logged := capture.requestID("recover boom")
		if !logged {
			t.Fatal("panic was not logged")
		}
		if got != "req-1" {
			t.Errorf("request_id on log ctx = %q, want req-1", got)
		}
	})

	t.Run("WithSSE producer", func(t *testing.T) {
		engine := sseEngines()["stdx"]()
		engine.Group("").GET("/sse", func(ctx httpx.Context) error {
			ctx.SetContext(reqCtx)
			return WithSSE(func(httpx.Context) (SSEStream[sseMsg], error) {
				return func(func(sseMsg) error) error { panic("boom") }, nil
			}, WithSSEHeartbeat(0))(ctx)
		})
		if status, _, body := sseDo(t, engine, "/sse"); status != http.StatusInternalServerError {
			t.Fatalf("status = %d, body=%q", status, body)
		}
		got, logged := capture.requestID("WithSSE stream panic")
		if !logged {
			t.Fatal("producer panic was not logged")
		}
		if got != "req-1" {
			t.Errorf("request_id on log ctx = %q, want req-1", got)
		}
	})
}
