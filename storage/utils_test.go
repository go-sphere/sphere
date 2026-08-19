package storage

import (
	"strings"
	"testing"
	"time"
)

func TestResolveUploadTTL(t *testing.T) {
	t.Parallel()

	const (
		configTTL  = 10 * time.Minute
		defaultTTL = time.Hour
	)
	tests := []struct {
		name      string
		reqTTL    time.Duration
		configTTL time.Duration
		want      time.Duration
	}{
		{name: "unset request uses config", reqTTL: 0, configTTL: configTTL, want: configTTL},
		{name: "unset request and config use default", reqTTL: 0, configTTL: 0, want: defaultTTL},
		{name: "shorter request is honored", reqTTL: time.Minute, configTTL: configTTL, want: time.Minute},
		// The ceiling is what keeps a client-supplied TTL from widening the window.
		{name: "longer request is clamped to config", reqTTL: 24 * time.Hour, configTTL: configTTL, want: configTTL},
		{name: "longer request is clamped to default", reqTTL: 24 * time.Hour, configTTL: 0, want: defaultTTL},
		{name: "negative request uses ceiling", reqTTL: -time.Minute, configTTL: configTTL, want: configTTL},
		{name: "request equal to ceiling stays", reqTTL: configTTL, configTTL: configTTL, want: configTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ResolveUploadTTL(tt.reqTTL, tt.configTTL, defaultTTL); got != tt.want {
				t.Fatalf("ResolveUploadTTL(%v, %v, %v) = %v, want %v", tt.reqTTL, tt.configTTL, defaultTTL, got, tt.want)
			}
		})
	}
}

func TestBuildUploadFileName(t *testing.T) {
	t.Run("default strategy is random ext", func(t *testing.T) {
		name1, err := BuildUploadFileName("avatar.png", "")
		if err != nil {
			t.Fatalf("BuildUploadFileName() error = %v", err)
		}
		name2, err := BuildUploadFileName("avatar.png", "")
		if err != nil {
			t.Fatalf("BuildUploadFileName() error = %v", err)
		}
		if !strings.HasSuffix(name1, ".png") || !strings.HasSuffix(name2, ".png") {
			t.Fatalf("unexpected ext: %q, %q", name1, name2)
		}
		if name1 == name2 {
			t.Fatalf("random names should differ, got %q", name1)
		}
	})

	t.Run("hash ext is stable", func(t *testing.T) {
		name1, err := BuildUploadFileName("avatar.png", UploadNamingStrategyHashExt)
		if err != nil {
			t.Fatalf("BuildUploadFileName() error = %v", err)
		}
		name2, err := BuildUploadFileName("avatar.png", UploadNamingStrategyHashExt)
		if err != nil {
			t.Fatalf("BuildUploadFileName() error = %v", err)
		}
		if name1 != "4a301072dec6b6a49050e5b294cd7983.png" {
			t.Fatalf("hash name = %q, want %q", name1, "4a301072dec6b6a49050e5b294cd7983.png")
		}
		if name1 != name2 {
			t.Fatalf("hash names should match: %q != %q", name1, name2)
		}
	})

	t.Run("original strategy sanitizes basename", func(t *testing.T) {
		name, err := BuildUploadFileName("user/avatar.png", UploadNamingStrategyOriginal)
		if err != nil {
			t.Fatalf("BuildUploadFileName() error = %v", err)
		}
		if name != "avatar.png" {
			t.Fatalf("name = %q, want %q", name, "avatar.png")
		}
	})

	t.Run("reject invalid input", func(t *testing.T) {
		_, err := BuildUploadFileName("", UploadNamingStrategyRandomExt)
		if err == nil {
			t.Fatal("expected error for empty file name, got nil")
		}
		_, err = BuildUploadFileName("avatar.png", UploadNamingStrategy("bad"))
		if err == nil {
			t.Fatal("expected error for unsupported strategy, got nil")
		}
		_, err = BuildUploadFileName("..", UploadNamingStrategyOriginal)
		if err == nil {
			t.Fatal("expected error for invalid original file name, got nil")
		}
	})
}

func TestJoinUploadKey(t *testing.T) {
	key, err := JoinUploadKey("/prefix", "users", "a.png")
	if err != nil {
		t.Fatalf("JoinUploadKey() error = %v", err)
	}
	if key != "prefix/users/a.png" {
		t.Fatalf("key = %q, want %q", key, "prefix/users/a.png")
	}

	key, err = JoinUploadKey("", "users", "a.png")
	if err != nil {
		t.Fatalf("JoinUploadKey() error = %v", err)
	}
	if key != "users/a.png" {
		t.Fatalf("key = %q, want %q", key, "users/a.png")
	}

	_, err = JoinUploadKey("prefix", "/users", "a.png")
	if err == nil {
		t.Fatal("expected error for absolute biz dir, got nil")
	}

	_, err = JoinUploadKey("prefix", "../users", "a.png")
	if err == nil {
		t.Fatal("expected error for parent biz dir, got nil")
	}

	_, err = JoinUploadKey("prefix", "a/../../users", "a.png")
	if err == nil {
		t.Fatal("expected error for traversal biz dir, got nil")
	}
}
