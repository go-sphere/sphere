package authparser

type AuthParser interface {
	ParseToken(token string) (*Claims, error)
	ParseRoles(roles string) []string
}

type Claims struct {
	Subject  string
	Username string
	Roles    string
	Exp      int64
}
