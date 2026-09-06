package redis

import (
	"testing"
)

func TestNewClientRejectsInvalidURL(t *testing.T) {
	_, err := NewClient(Config{URL: "://not-a-url"})
	if err == nil {
		t.Fatal("NewClient with an invalid URL returned nil error")
	}
}

func TestNewClientParsesValidURL(t *testing.T) {
	client, err := NewClient(Config{URL: "redis://127.0.0.1:6379/0"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned a nil client for a valid URL")
	}
	_ = client.Close()
}
