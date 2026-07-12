package secure

import (
	"strings"
	"testing"
)

func TestCryptPasswordAndSalt(t *testing.T) {
	raw := "12345678"
	pwd1, err := CryptPassword(raw)
	if err != nil {
		t.Fatalf("crypt password failed: %v", err)
	}
	pwd2, err := CryptPassword(raw)
	if err != nil {
		t.Fatalf("crypt password failed: %v", err)
	}
	t.Log(pwd1, pwd2)
	if !IsPasswordMatch(raw, pwd1) {
		t.Error("password not match")
	}
	if !IsPasswordMatch(raw, pwd2) {
		t.Error("password not match")
	}
}

func TestCryptPasswordTooLong(t *testing.T) {
	raw := strings.Repeat("a", 73)
	hash, err := CryptPassword(raw)
	if err == nil {
		t.Fatal("expected error for over-length password")
	}
	if hash != "" {
		t.Errorf("expected empty hash on error, got %q", hash)
	}
}
