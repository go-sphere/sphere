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
	if pwd1 == pwd2 {
		t.Fatal("CryptPassword reused a salt")
	}
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

func TestIsPasswordMatchLongerThan72Bytes(t *testing.T) {
	// 72-byte ASCII password
	pwd72 := strings.Repeat("k", 72)
	hash72, err := CryptPassword(pwd72)
	if err != nil {
		t.Fatalf("CryptPassword(72 bytes) failed: %v", err)
	}
	if !IsPasswordMatch(pwd72, hash72) {
		t.Fatal("IsPasswordMatch(72 bytes) expected true")
	}

	// Candidates exceeding 72 bytes that prefix-match the 72-byte password
	// must be strictly rejected (bcrypt internal truncation defense).
	overCandidates := []string{
		pwd72 + "x",
		pwd72 + "12345",
		pwd72 + strings.Repeat("z", 100),
	}
	for _, cand := range overCandidates {
		if IsPasswordMatch(cand, hash72) {
			t.Errorf("IsPasswordMatch(len=%d) matched hash of 72-byte password; want false", len(cand))
		}
	}

	// 72-byte UTF-8 multi-byte password (24 * 3 bytes = 72 bytes)
	unicode72 := strings.Repeat("密", 24)
	uHash72, err := CryptPassword(unicode72)
	if err != nil {
		t.Fatalf("CryptPassword(unicode 72 bytes) failed: %v", err)
	}
	if !IsPasswordMatch(unicode72, uHash72) {
		t.Fatal("IsPasswordMatch(unicode 72 bytes) expected true")
	}

	// 75-byte UTF-8 candidate sharing first 72 bytes
	unicode75 := unicode72 + "码" // 72 + 3 = 75 bytes
	if IsPasswordMatch(unicode75, uHash72) {
		t.Errorf("IsPasswordMatch(unicode len=%d) matched hash of 72-byte password; want false", len(unicode75))
	}
}

func TestIsPasswordMatchRejectsInvalidHash(t *testing.T) {
	for _, hash := range []string{"", "not-a-bcrypt-hash", "$2a$bad"} {
		if IsPasswordMatch("password", hash) {
			t.Errorf("IsPasswordMatch accepted invalid hash %q", hash)
		}
	}
}
