package secure

import (
	"golang.org/x/crypto/bcrypt"
)

func CryptPassword(pwd string) string {
	cyPwd, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return pwd
	}
	return string(cyPwd)
}

func IsPasswordMatch(pwd, cyPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(cyPwd), []byte(pwd))
	return err == nil
}
