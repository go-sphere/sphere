package redis

import (
	"strings"
	"testing"
)

func TestNewClientRejectsInvalidURL(t *testing.T) {
	_, err := NewClient(Config{URL: "://not-a-url"})
	if err == nil {
		t.Fatal("NewClient with an invalid URL returned nil error")
	}
}

func TestNewClientRedactsCredentialsOnInvalidURLs(t *testing.T) {
	const secret = "super-secret-pass"
	for _, bad := range []string{
		"redis://alice:" + secret + "@127.0.0.1:badport/0",
		"redis:/alice:" + secret + "@127.0.0.1:6379/0",
		"redis:/:" + secret + "@127.0.0.1:6379",
		"rediss:/alice:" + secret + "@localhost/0",
	} {
		_, err := NewClient(Config{URL: bad})
		if err == nil {
			t.Fatalf("NewClient(%q) succeeded, want error", bad)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("NewClient(%q) error leaked secret: %q", bad, err.Error())
		}
	}
}

func TestNewClientParsesValidURL(t *testing.T) {
	for _, valid := range []string{"redis://127.0.0.1:1/0", "redis:///0"} {
		client, err := NewClient(Config{URL: valid})
		if err != nil {
			t.Fatalf("NewClient(%q) must not dial the server: %v", valid, err)
		}
		if client == nil {
			t.Fatalf("NewClient(%q) returned a nil client for a valid URL", valid)
		}
		_ = client.Close()
	}
}
