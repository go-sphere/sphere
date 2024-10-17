package secure

import "testing"

func TestCryptPasswordAndSalt(t *testing.T) {
	raw := "12345678"
	pwd1 := CryptPassword(raw)
	pwd2 := CryptPassword(raw)
	t.Log(pwd1, pwd2)
	if !IsPasswordMatch(raw, pwd1) {
		t.Error("password not match")
	}
	if !IsPasswordMatch(raw, pwd2) {
		t.Error("password not match")
	}
}
