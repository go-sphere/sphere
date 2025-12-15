package httpx

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestListenAndAutoShutdown(t *testing.T) {
	if os.Getenv("SPHERE_TEST_LISTEN_AUTO_SHUTDOWN") != "1" {
		t.Skip("Skipping ListenAndAutoShutdown test, set SPHERE_TEST_LISTEN_AUTO_SHUTDOWN=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := &http.Server{
		Addr: "localhost:0",
	}
	err := ListenAndAutoShutdown(ctx, server, 2*time.Second)
	if err != nil {
		t.Fatalf("ListenAndAutoShutdown failed: %v", err)
	}
}
