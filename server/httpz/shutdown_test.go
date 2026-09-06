package httpz

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestStopServerNil(t *testing.T) {
	if err := StopServer(t.Context(), nil); err != nil {
		t.Fatalf("StopServer(nil) = %v", err)
	}
}

func TestStopServerIdleReturnsNil(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := StopServer(ctx, srv); err != nil {
		t.Fatalf("idle StopServer = %v", err)
	}
}

func TestStopServerForceClosesHungRequest(t *testing.T) {
	started := make(chan struct{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	clientDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			clientDone <- err
			return
		}
		_ = resp.Body.Close()
		clientDone <- nil
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	if err := StopServer(ctx, srv); err != nil {
		t.Fatalf("StopServer after timeout = %v, want nil after force close", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("StopServer took %v, want shutdown timeout then Close", elapsed)
	}
	select {
	case err := <-clientDone:
		if err == nil {
			t.Fatal("hung client should error after force close")
		}
	case <-time.After(time.Second):
		t.Fatal("hung client still blocked after StopServer")
	}
}
