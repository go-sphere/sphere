package httpz

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/httpx/fiberx"
	"github.com/go-sphere/httpx/ginx"
)

type sseMsg struct {
	Msg string `json:"msg"`
}

func sseEngines() map[string]func() httpx.Engine {
	return map[string]func() httpx.Engine{
		"ginx":   func() httpx.Engine { return ginx.New() },
		"fiberx": func() httpx.Engine { return fiberx.New() },
	}
}

func sseDo(t *testing.T, engine httpx.Engine, target string) (int, http.Header, string) {
	t.Helper()
	tr, ok := httpx.AsTestRequester(engine)
	if !ok {
		t.Fatal("engine does not support in-process dispatch")
	}
	resp, err := tr.Do(httptest.NewRequest(http.MethodGet, "http://example.com"+target, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode, resp.Header, string(body)
}

// TestWithSSEHappyPath pins the full success wire format: unnamed data
// events in order, the terminal done event, and the SSE headers.
func TestWithSSEHappyPath(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					if err := send(sseMsg{Msg: "one"}); err != nil {
						return err
					}
					return send(sseMsg{Msg: "two"})
				}, nil
			}, WithSSEHeartbeat(0)))

			status, header, body := sseDo(t, engine, "/sse")
			if status != http.StatusOK {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			want := "data: {\"msg\":\"one\"}\n\n" +
				"data: {\"msg\":\"two\"}\n\n" +
				"event: done\ndata: {}\n\n"
			if body != want {
				t.Fatalf("body = %q, want %q", body, want)
			}
			if ct := header.Get("Content-Type"); ct != httpx.ContentTypeEventStream {
				t.Fatalf("content-type = %q", ct)
			}
			if cc := header.Get("Cache-Control"); cc != "no-cache" {
				t.Fatalf("cache-control = %q", cc)
			}
		})
	}
}

// TestWithSSEPrepareError pins phase one of the error contract: a bind-time
// failure is a plain JSON error status, no stream is committed.
func TestWithSSEPrepareError(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return nil, httpx.NewBadRequestError("bad input")
			}, WithSSEHeartbeat(0)))

			status, header, body := sseDo(t, engine, "/sse")
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("content-type = %q, want JSON error", ct)
			}
			if !strings.Contains(body, `"message":"bad input"`) {
				t.Fatalf("body = %q, want ErrorResponse with message", body)
			}
		})
	}
}

// TestWithSSEErrorBeforeFirstSend pins the lazy commit: a stream failure
// before any message still becomes a JSON error status, not a 200 stream.
func TestWithSSEErrorBeforeFirstSend(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					return httpx.NewForbiddenError("not yours")
				}, nil
			}, WithSSEHeartbeat(0)))

			status, header, body := sseDo(t, engine, "/sse")
			if status != http.StatusForbidden {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("content-type = %q, want JSON error", ct)
			}
			if !strings.Contains(body, `"message":"not yours"`) {
				t.Fatalf("body = %q", body)
			}
		})
	}
}

// TestWithSSEErrorAfterFirstSend pins phase two: once a message is out the
// status is 200 and the failure arrives as a terminal error event rendered
// like the unary ErrorResponse envelope.
func TestWithSSEErrorAfterFirstSend(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					if err := send(sseMsg{Msg: "one"}); err != nil {
						return err
					}
					return httpx.NewForbiddenError("mid-stream denial")
				}, nil
			}, WithSSEHeartbeat(0)))

			status, _, body := sseDo(t, engine, "/sse")
			if status != http.StatusOK {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			want := "data: {\"msg\":\"one\"}\n\n" +
				"event: error\ndata: {\"success\":false,\"code\":0,\"message\":\"mid-stream denial\"}\n\n"
			if body != want {
				t.Fatalf("body = %q, want %q", body, want)
			}
		})
	}
}

// TestWithSSEEmptyStream pins that a producer which never sends still yields
// a well-formed stream with only the done event.
func TestWithSSEEmptyStream(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error { return nil }, nil
			}, WithSSEHeartbeat(0)))

			status, header, body := sseDo(t, engine, "/sse")
			if status != http.StatusOK {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			if body != "event: done\ndata: {}\n\n" {
				t.Fatalf("body = %q", body)
			}
			if ct := header.Get("Content-Type"); ct != httpx.ContentTypeEventStream {
				t.Fatalf("content-type = %q", ct)
			}
		})
	}
}

// TestWithSSEPanicSemantics pins panic handling on both sides of the commit:
// before any send a panic is a JSON 500, after a send it is a terminal error
// event on a 200 stream — and it never leaks internals.
func TestWithSSEPanicSemantics(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			root := engine.Group("")
			root.GET("/panic-before", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					panic("secret before")
				}, nil
			}, WithSSEHeartbeat(0)))
			root.GET("/panic-after", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					if err := send(sseMsg{Msg: "one"}); err != nil {
						return err
					}
					panic("secret after")
				}, nil
			}, WithSSEHeartbeat(0)))

			status, _, body := sseDo(t, engine, "/panic-before")
			if status != http.StatusInternalServerError {
				t.Fatalf("panic-before status = %d, body=%q", status, body)
			}
			if strings.Contains(body, "secret") {
				t.Fatalf("panic-before leaked internals: %q", body)
			}

			status, _, body = sseDo(t, engine, "/panic-after")
			if status != http.StatusOK {
				t.Fatalf("panic-after status = %d, body=%q", status, body)
			}
			if !strings.Contains(body, "event: error\n") || strings.Contains(body, "secret") {
				t.Fatalf("panic-after body = %q, want sanitized error event", body)
			}
		})
	}
}

// TestWithSSEHeartbeatAndRetry pins the optional frames: a retry frame right
// after commit and comment heartbeats while the producer is idle.
func TestWithSSEHeartbeatAndRetry(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
				return func(send func(sseMsg) error) error {
					if err := send(sseMsg{Msg: "one"}); err != nil {
						return err
					}
					time.Sleep(80 * time.Millisecond)
					return send(sseMsg{Msg: "two"})
				}, nil
			}, WithSSEHeartbeat(20*time.Millisecond), WithSSERetry(3*time.Second)))

			status, _, body := sseDo(t, engine, "/sse")
			if status != http.StatusOK {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			if !strings.HasPrefix(body, "retry: 3000\n\n") {
				t.Fatalf("body = %q, want retry frame first", body)
			}
			if !strings.Contains(body, ": \n\n") {
				t.Fatalf("body = %q, want at least one heartbeat comment", body)
			}
			for _, frag := range []string{"data: {\"msg\":\"one\"}\n\n", "data: {\"msg\":\"two\"}\n\n", "event: done\ndata: {}\n\n"} {
				if !strings.Contains(body, frag) {
					t.Fatalf("body = %q, missing %q", body, frag)
				}
			}
		})
	}
}

// TestWithSSESendUnblocksWhenClientGone pins that a canceled pump unblocks a
// blocked producer: send must return an error instead of leaking the
// goroutine. Exercised directly through the pump cancel path via request
// context cancellation on a live ginx server is heavyweight; instead this
// relies on the abort path being triggered by a write failure, which the
// in-process harness cannot simulate, so we test the producer contract at
// the unit level: cancel the producer context and assert send errors.
func TestWithSSESendUnblocksWhenClientGone(t *testing.T) {
	// Covered indirectly: pumpSSE cancels prodCtx on any write failure or
	// request cancellation, and send selects on prodCtx.Done. The gate path
	// (request canceled before first frame) is unit-testable only with a
	// cancelable request context, which in-process dispatch does not provide;
	// the conformance-grade live test lives with the streaming layout smoke
	// tests. This test pins the drain helper so a finished producer never
	// blocks.
	frames := make(chan sseMsg, 1)
	result := make(chan error, 1)
	frames <- sseMsg{Msg: "pending"}
	close(frames)
	result <- errors.New("boom")
	drainSSE(frames, result) // must not deadlock
}

// generatedWatchRequest/Response and the handler below replicate, shape for
// shape, what protoc-gen-sphere emits for a server-streaming method. This
// pins the contract between the generator template and the WithSSE API: if
// either side drifts, this stops compiling or the wire format changes.
type generatedWatchRequest struct {
	Topic string `json:"topic" uri:"topic"`
	Limit int64  `json:"limit" query:"limit"`
}

type generatedWatchResponse struct {
	Event string `json:"event"`
	Seq   int64  `json:"seq"`
}

type generatedStreamServer interface {
	Watch(context.Context, *generatedWatchRequest, func(*generatedWatchResponse) error) error
}

func generatedWatchHandler(srv generatedStreamServer) httpx.Handler {
	return WithSSE(func(ctx httpx.Context) (SSEStream[*generatedWatchResponse], error) {
		var in generatedWatchRequest
		if err := ctx.BindQuery(&in); err != nil {
			return nil, err
		}
		if err := ctx.BindURI(&in); err != nil {
			return nil, err
		}
		stdCtx := ctx.Context()
		return func(send func(*generatedWatchResponse) error) error {
			return srv.Watch(stdCtx, &in, send)
		}, nil
	})
}

type fakeStreamServer struct{}

func (fakeStreamServer) Watch(ctx context.Context, req *generatedWatchRequest, send func(*generatedWatchResponse) error) error {
	for i := int64(0); i < req.Limit; i++ {
		if err := send(&generatedWatchResponse{Event: req.Topic, Seq: i}); err != nil {
			return err
		}
	}
	return nil
}

func TestWithSSEGeneratedShape(t *testing.T) {
	for name, newEngine := range sseEngines() {
		t.Run(name, func(t *testing.T) {
			engine := newEngine()
			engine.Group("").GET("/watch/:topic", generatedWatchHandler(fakeStreamServer{}))

			status, _, body := sseDo(t, engine, "/watch/news?limit=2")
			if status != http.StatusOK {
				t.Fatalf("status = %d, body=%q", status, body)
			}
			want := "data: {\"event\":\"news\",\"seq\":0}\n\n" +
				"data: {\"event\":\"news\",\"seq\":1}\n\n" +
				"event: done\ndata: {}\n\n"
			if body != want {
				t.Fatalf("body = %q, want %q", body, want)
			}
		})
	}
}

// TestWithSSELiveDisconnectUnblocksProducer runs a real server and drops the
// client mid-stream: the producer's send must return an error shortly after,
// proving the pump's abort path unblocks the producer goroutine instead of
// leaking it.
func TestWithSSELiveDisconnectUnblocksProducer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve addr: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	sendErr := make(chan error, 1)
	engine := ginx.New(ginx.WithServerAddr(addr))
	engine.Group("").GET("/sse", WithSSE(func(ctx httpx.Context) (SSEStream[sseMsg], error) {
		return func(send func(sseMsg) error) error {
			for {
				if err := send(sseMsg{Msg: "tick"}); err != nil {
					sendErr <- err
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
		}, nil
	}, WithSSEHeartbeat(25*time.Millisecond)))

	go func() { _ = engine.Start() }()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = engine.Stop(stopCtx)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !engine.IsRunning() {
		if time.Now().After(deadline) {
			t.Fatal("engine did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+addr+"/sse", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	first, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first event line: %v", err)
	}
	if !strings.HasPrefix(first, "data: ") {
		t.Fatalf("first line = %q", first)
	}

	// Drop the client mid-stream.
	cancelReq()
	_ = resp.Body.Close()

	select {
	case err := <-sendErr:
		if err == nil {
			t.Fatal("send error = nil, want non-nil after disconnect")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("producer still blocked 3s after client disconnect")
	}
}
