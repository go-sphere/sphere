package storage

import (
	"strings"
	"testing"
)

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
