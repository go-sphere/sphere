package encrypt

import (
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"time"
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

func CryptPasswordAndSalt(password string) (string, string) {
	id, err := uuid.NewUUID()
	salt := ""
	if err != nil {
		salt = fmt.Sprintf("%d", time.Now().Unix())[0:8]
	} else {
		salt = id.String()[0:8]
	}
	password = CryptPassword(password + salt)
	return password, salt
}

func IsPasswordAndSaltMatch(password, salt, cyPwd string) bool {
	return IsPasswordMatch(password+salt, cyPwd)
}
