package encrypt

import (
	"golang.org/x/crypto/bcrypt"
)

func IsPasswordMatch(pwd, cyPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(cyPwd), []byte(pwd))
	return err == nil
}

func CryptPassword(pwd string) string {
	cyPwd, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return pwd
	}
	return string(cyPwd)
}
