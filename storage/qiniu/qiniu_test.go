package qiniu

import (
	"testing"

	qiniuStorage "github.com/qiniu/go-sdk/v7/storage"
)

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	err := &qiniuStorage.ErrorInfo{Code: 612}
	if !isNotFoundError(err) {
		t.Fatalf("expected Qiniu error code 612 to be recognized as not found")
	}
}

func TestNewClientMimeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf string
		want string
	}{
		{name: "empty takes the restrictive default", conf: "", want: DefaultMimeLimit},
		{name: "explicit opt out clears the policy", conf: AnyMimeLimit, want: ""},
		{name: "custom value is preserved", conf: "application/pdf", want: "application/pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(Config{PublicBase: "https://cdn.example.com", MimeLimit: tt.conf})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if client.config.MimeLimit != tt.want {
				t.Fatalf("MimeLimit = %q, want %q", client.config.MimeLimit, tt.want)
			}
		})
	}
}
