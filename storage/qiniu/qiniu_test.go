package qiniu

import (
	"errors"
	"fmt"
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

// TestIsDownloadNotFoundError pins that a missing object is recognised on the
// download path too. Downloads are served by the object source and answer with
// a plain HTTP 404, while the bucket-management API answers with Qiniu's 612;
// matching only 612 left every missing download reported as a transport
// failure, which an HTTP handler turns into a 500 instead of a 404.
func TestIsDownloadNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		download bool
		manage   bool
	}{
		{name: "nil", err: nil},
		{name: "rs missing key (612)", err: &qiniuStorage.ErrorInfo{Code: 612}, download: true, manage: true},
		{name: "download missing key (404)", err: &qiniuStorage.ErrorInfo{Code: 404}, download: true},
		{name: "wrapped download 404", err: fmt.Errorf("get object: %w", &qiniuStorage.ErrorInfo{Code: 404}), download: true},
		{name: "server error", err: &qiniuStorage.ErrorInfo{Code: 599}},
		{name: "forbidden", err: &qiniuStorage.ErrorInfo{Code: 403}},
		{name: "unrelated error", err: errors.New("connection reset")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDownloadNotFoundError(tt.err); got != tt.download {
				t.Errorf("isDownloadNotFoundError() = %v, want %v", got, tt.download)
			}
			if got := isNotFoundError(tt.err); got != tt.manage {
				t.Errorf("isNotFoundError() = %v, want %v", got, tt.manage)
			}
		})
	}
}
