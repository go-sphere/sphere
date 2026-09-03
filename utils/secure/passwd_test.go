package secure

import (
	"fmt"
	"strings"
	"sync"
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

// TestIsPasswordMatchAdversarial verifies IsPasswordMatch against all edge cases:
// 72B, 73B, 100B, 1000B, prefix collision attacks, UTF-8 rune boundaries, null bytes,
// malformed hashes, and high concurrency under -race.
func TestIsPasswordMatchAdversarial(t *testing.T) {
	t.Parallel()

	t.Run("boundary candidates against 72B hash", func(t *testing.T) {
		t.Parallel()
		pwd72 := strings.Repeat("a", 72)
		hash72, err := CryptPassword(pwd72)
		if err != nil {
			t.Fatalf("CryptPassword(72 bytes) failed: %v", err)
		}

		// Exact match
		if !IsPasswordMatch(pwd72, hash72) {
			t.Fatal("IsPasswordMatch(72 bytes, exact) must be true")
		}

		// Prefix attacks: candidates extending 72 bytes
		overCandidates := []struct {
			name string
			cand string
		}{
			{"73 bytes (+1)", pwd72 + "b"},
			{"74 bytes (+2)", pwd72 + "bc"},
			{"75 bytes (+3)", pwd72 + "bcd"},
			{"80 bytes (+8)", pwd72 + "12345678"},
			{"100 bytes (+28)", pwd72 + strings.Repeat("z", 28)},
			{"128 bytes (+56)", pwd72 + strings.Repeat("x", 56)},
			{"256 bytes (+184)", pwd72 + strings.Repeat("w", 184)},
			{"1000 bytes (+928)", pwd72 + strings.Repeat("q", 928)},
			{"10000 bytes", pwd72 + strings.Repeat("k", 9928)},
		}

		for _, tc := range overCandidates {
			if IsPasswordMatch(tc.cand, hash72) {
				t.Errorf("[%s] IsPasswordMatch returned true for %d-byte candidate with 72-byte prefix match; must be false", tc.name, len(tc.cand))
			}
		}

		// Non-matching prefixes
		nonMatching := []struct {
			name string
			cand string
		}{
			{"72 bytes different", strings.Repeat("b", 72)},
			{"73 bytes different", strings.Repeat("b", 73)},
			{"100 bytes different", strings.Repeat("b", 100)},
			{"1000 bytes different", strings.Repeat("b", 1000)},
			{"empty candidate", ""},
			{"1 byte", "a"},
			{"71 bytes prefix", pwd72[:71]},
		}

		for _, tc := range nonMatching {
			if IsPasswordMatch(tc.cand, hash72) {
				t.Errorf("[%s] IsPasswordMatch returned true for %d-byte non-matching candidate; must be false", tc.name, len(tc.cand))
			}
		}
	})

	t.Run("prefix attacks against short passwords", func(t *testing.T) {
		t.Parallel()
		shortPwd := "Secret123!"
		hashShort, err := CryptPassword(shortPwd)
		if err != nil {
			t.Fatalf("CryptPassword(short) failed: %v", err)
		}

		if !IsPasswordMatch(shortPwd, hashShort) {
			t.Fatal("IsPasswordMatch(shortPwd) must be true")
		}

		candidates := []string{
			shortPwd + "x",
			shortPwd + strings.Repeat("0", 62),  // 72 bytes total
			shortPwd + strings.Repeat("0", 63),  // 73 bytes total
			shortPwd + strings.Repeat("0", 90),  // 100 bytes total
			shortPwd + strings.Repeat("0", 990), // 1000 bytes total
		}

		for _, cand := range candidates {
			if IsPasswordMatch(cand, hashShort) {
				t.Errorf("IsPasswordMatch returned true for candidate %q (len=%d) against short hash", cand, len(cand))
			}
		}
	})

	t.Run("utf8 multi-byte rune boundaries", func(t *testing.T) {
		t.Parallel()

		// 1. 3-byte Chinese characters: 24 runes = 72 bytes
		cn72 := strings.Repeat("密", 24)
		cnHash72, err := CryptPassword(cn72)
		if err != nil {
			t.Fatalf("CryptPassword(cn72) failed: %v", err)
		}
		if !IsPasswordMatch(cn72, cnHash72) {
			t.Fatal("IsPasswordMatch(cn72) must be true")
		}

		// Multi-byte candidates extending beyond 72 bytes
		cnOverCandidates := []struct {
			name string
			cand string
		}{
			{"73 bytes (+1 ASCII)", cn72 + "x"},
			{"74 bytes (+2 bytes Greek)", cn72 + "Ω"},
			{"75 bytes (+3 bytes Chinese)", cn72 + "码"},
			{"76 bytes (+4 bytes Emoji)", cn72 + "🔒"},
			{"81 bytes (+3 Chinese runes)", cn72 + "一二三"},
		}
		for _, tc := range cnOverCandidates {
			if IsPasswordMatch(tc.cand, cnHash72) {
				t.Errorf("[%s] IsPasswordMatch returned true for len=%d bytes UTF-8 candidate; must be false", tc.name, len([]byte(tc.cand)))
			}
		}

		// 2. 2-byte Greek characters: 36 runes = 72 bytes
		greek72 := strings.Repeat("θ", 36)
		greekHash72, err := CryptPassword(greek72)
		if err != nil {
			t.Fatalf("CryptPassword(greek72) failed: %v", err)
		}
		if !IsPasswordMatch(greek72, greekHash72) {
			t.Fatal("IsPasswordMatch(greek72) must be true")
		}
		if IsPasswordMatch(greek72+"α", greekHash72) { // 74 bytes
			t.Error("IsPasswordMatch returned true for 74-byte Greek candidate")
		}

		// 3. 4-byte Emojis: 18 emojis = 72 bytes
		emoji4Byte := strings.Repeat("🦀", 18) // 18 * 4 = 72 bytes
		if len([]byte(emoji4Byte)) != 72 {
			t.Fatalf("emoji4Byte length is %d, expected 72", len([]byte(emoji4Byte)))
		}
		emojiHash72, err := CryptPassword(emoji4Byte)
		if err != nil {
			t.Fatalf("CryptPassword(emoji4Byte) failed: %v", err)
		}
		if !IsPasswordMatch(emoji4Byte, emojiHash72) {
			t.Fatal("IsPasswordMatch(emoji4Byte) must be true")
		}
		if IsPasswordMatch(emoji4Byte+"🦀", emojiHash72) { // 76 bytes
			t.Error("IsPasswordMatch returned true for 76-byte Emoji candidate")
		}

		// 4. Boundary crossing: 71 ASCII bytes + 2-byte rune = 73 bytes
		prefix71 := strings.Repeat("k", 71)
		hash71, err := CryptPassword(prefix71)
		if err != nil {
			t.Fatalf("CryptPassword(71 bytes) failed: %v", err)
		}
		if !IsPasswordMatch(prefix71, hash71) {
			t.Fatal("IsPasswordMatch(71 bytes) must be true")
		}
		if IsPasswordMatch(prefix71+"k", hash71) { // 72 bytes, different
			t.Error("IsPasswordMatch returned true for 72-byte candidate against 71-byte hash")
		}
		if IsPasswordMatch(prefix71+"π", hash71) { // 71 + 2 = 73 bytes
			t.Error("IsPasswordMatch returned true for 73-byte candidate (crossing boundary)")
		}
		if IsPasswordMatch(prefix71+"汉", hash71) { // 71 + 3 = 74 bytes
			t.Error("IsPasswordMatch returned true for 74-byte candidate (crossing boundary)")
		}
	})

	t.Run("null bytes and binary sequences", func(t *testing.T) {
		t.Parallel()
		rawNull := "pass\x00word\x00123"
		hashNull, err := CryptPassword(rawNull)
		if err != nil {
			t.Fatalf("CryptPassword with null bytes failed: %v", err)
		}
		if !IsPasswordMatch(rawNull, hashNull) {
			t.Fatal("IsPasswordMatch with null bytes must be true")
		}
		if IsPasswordMatch("pass\x00word\x00124", hashNull) {
			t.Fatal("IsPasswordMatch matched altered null byte candidate")
		}
		if IsPasswordMatch("pass", hashNull) {
			t.Fatal("IsPasswordMatch truncated at first null byte")
		}

		// 72 bytes with null bytes
		null72 := strings.Repeat("a\x00", 36)
		nullHash72, err := CryptPassword(null72)
		if err != nil {
			t.Fatalf("CryptPassword(null72) failed: %v", err)
		}
		if !IsPasswordMatch(null72, nullHash72) {
			t.Fatal("IsPasswordMatch(null72) must be true")
		}
		if IsPasswordMatch(null72+"\x00", nullHash72) { // 73 bytes
			t.Fatal("IsPasswordMatch(73 bytes with null) must be false")
		}
	})

	t.Run("malformed and invalid hashes", func(t *testing.T) {
		t.Parallel()
		invalidHashes := []string{
			"",
			"invalid-hash",
			"$2a$10$",
			"$2a$10$not-enough-characters",
			"$2y$12$01234567890123456789012345678901234567890123456789012",
			strings.Repeat("?", 100),
		}

		for _, badHash := range invalidHashes {
			if IsPasswordMatch("some-password", badHash) {
				t.Errorf("IsPasswordMatch returned true for invalid hash %q", badHash)
			}
			if IsPasswordMatch("", badHash) {
				t.Errorf("IsPasswordMatch returned true for empty pwd and invalid hash %q", badHash)
			}
		}
	})

	t.Run("high concurrency goroutines under -race", func(t *testing.T) {
		t.Parallel()
		const goroutines = 20
		const iterations = 1

		passwords := []string{
			"",
			"simple",
			"a-longer-password-12345",
			strings.Repeat("x", 71),
			strings.Repeat("y", 72),
			strings.Repeat("密", 24), // 72 bytes
			strings.Repeat("🦀", 18), // 72 bytes
		}

		hashes := make([]string, len(passwords))
		for i, p := range passwords {
			h, err := CryptPassword(p)
			if err != nil {
				t.Fatalf("CryptPassword(%d) failed: %v", i, err)
			}
			hashes[i] = h
		}

		var wg sync.WaitGroup
		wg.Add(goroutines)

		errCh := make(chan error, goroutines*iterations)

		for g := range goroutines {
			go func(gid int) {
				defer wg.Done()
				for iter := range iterations {
					idx := (gid + iter) % len(passwords)
					pwd := passwords[idx]
					hash := hashes[idx]

					// 1. Valid match
					if !IsPasswordMatch(pwd, hash) {
						errCh <- fmt.Errorf("worker %d iter %d: expected match for index %d", gid, iter, idx)
						return
					}

					// 2. Overlength candidate >72 bytes
					overCand := pwd + strings.Repeat("z", 73)
					if IsPasswordMatch(overCand, hash) {
						errCh <- fmt.Errorf("worker %d iter %d: overlength candidate matched for index %d", gid, iter, idx)
						return
					}

					// 3. Mismatched hash
					wrongIdx := (idx + 1) % len(passwords)
					wrongHash := hashes[wrongIdx]
					if pwd != passwords[wrongIdx] && IsPasswordMatch(pwd, wrongHash) {
						errCh <- fmt.Errorf("worker %d iter %d: wrong hash matched (%d vs %d)", gid, iter, idx, wrongIdx)
						return
					}
				}
			}(g)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatal(err)
		}
	})
}

// TestPasswordStress tests CryptPassword and IsPasswordMatch across boundaries, Unicode, and concurrency.
func TestPasswordStress(t *testing.T) {
	t.Run("large passwords over 72 bytes", func(t *testing.T) {
		overLengths := []int{73, 74, 100, 500, 1024, 10000}
		for _, l := range overLengths {
			raw := strings.Repeat("A", l)
			hash, err := CryptPassword(raw)
			if err == nil {
				t.Fatalf("CryptPassword len %d: expected error for >72 bytes", l)
			}
			if hash != "" {
				t.Fatalf("CryptPassword len %d: hash must be empty on error, got %q", l, hash)
			}
			// IsPasswordMatch on hash should be false
			if IsPasswordMatch(raw, hash) {
				t.Fatalf("IsPasswordMatch on empty hash returned true for len %d", l)
			}
		}
	})

	t.Run("empty password", func(t *testing.T) {
		raw := ""
		hash, err := CryptPassword(raw)
		if err != nil {
			t.Fatalf("CryptPassword empty: unexpected error: %v", err)
		}
		if hash == "" {
			t.Fatal("CryptPassword empty: expected non-empty bcrypt hash")
		}
		if !IsPasswordMatch(raw, hash) {
			t.Fatal("IsPasswordMatch empty password failed")
		}
		if IsPasswordMatch("not-empty", hash) {
			t.Fatal("IsPasswordMatch matched incorrect password on empty hash")
		}
	})

	t.Run("unicode passwords and byte boundaries", func(t *testing.T) {
		unicodeTests := []struct {
			name       string
			pwd        string
			expectOk   bool
			alteredPwd string
		}{
			{"chinese standard", "我的超级安全密码123!@#", true, "我的超级安全密码123!@#wrong"},
			{"emojis", "🔑🔐🛡️🚀🔥✨", true, "wrong🔑🔐🛡️🚀🔥✨"},
			{"mixed unicode exact 72 bytes", strings.Repeat("密", 24), true, "改" + strings.Repeat("密", 23)}, // 24 * 3 = 72 bytes, altered first rune
			{"mixed unicode 75 bytes", strings.Repeat("密", 25), false, ""},                                 // 25 * 3 = 75 bytes (> 72 bytes)
		}

		for _, tc := range unicodeTests {
			t.Run(tc.name, func(t *testing.T) {
				hash, err := CryptPassword(tc.pwd)
				if tc.expectOk {
					if err != nil {
						t.Fatalf("CryptPassword(%q) unexpected error: %v", tc.pwd, err)
					}
					if !IsPasswordMatch(tc.pwd, hash) {
						t.Fatalf("IsPasswordMatch failed for unicode password %q", tc.pwd)
					}
					if tc.alteredPwd != "" && IsPasswordMatch(tc.alteredPwd, hash) {
						t.Fatalf("IsPasswordMatch matched altered unicode password %q", tc.alteredPwd)
					}
				} else {
					if err == nil {
						t.Fatalf("CryptPassword(%q, %d bytes) expected error", tc.pwd, len(tc.pwd))
					}
					if hash != "" {
						t.Fatalf("CryptPassword(%q) expected empty hash on error, got %q", tc.pwd, hash)
					}
				}
			})
		}
	})

	t.Run("bcrypt 72 byte truncation behavior verification", func(t *testing.T) {
		// Validating bcrypt specification: exactly 72 bytes is the limit for GenerateFromPassword.
		pwd72 := strings.Repeat("x", 72)
		hash72, err := CryptPassword(pwd72)
		if err != nil {
			t.Fatalf("CryptPassword(72 bytes) failed: %v", err)
		}
		if !IsPasswordMatch(pwd72, hash72) {
			t.Fatal("IsPasswordMatch(72 bytes) failed")
		}

		// Appending characters beyond 72 bytes to candidate: bcrypt only hashes first 72 bytes during comparison
		// while CryptPassword correctly refuses to generate hashes for >72 bytes.
		pwd73 := pwd72 + "y"
		_, err73 := CryptPassword(pwd73)
		if err73 == nil {
			t.Fatal("CryptPassword(73 bytes) must return error")
		}
	})

	t.Run("concurrency 50 goroutines", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		errCh := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(id int) {
				defer wg.Done()
				pwd := fmt.Sprintf("pass-worker-%d-🚀", id)
				hash, err := CryptPassword(pwd)
				if err != nil {
					errCh <- fmt.Errorf("worker %d CryptPassword failed: %w", id, err)
					return
				}
				if !IsPasswordMatch(pwd, hash) {
					errCh <- fmt.Errorf("worker %d IsPasswordMatch failed", id)
					return
				}
				if IsPasswordMatch(pwd+"extra", hash) {
					errCh <- fmt.Errorf("worker %d IsPasswordMatch false positive", id)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Fatal(err)
		}
	})
}
