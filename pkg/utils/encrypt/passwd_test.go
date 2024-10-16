package encrypt

import "testing"

func TestCryptPasswordAndSalt(t *testing.T) {
	raw := "12345678"
	pwd, salt := CryptPasswordAndSalt(raw)
	if IsPasswordAndSaltMatch(raw, salt, pwd) {
		t.Log(pwd, salt)
	}
}
