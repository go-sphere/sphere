package auth

type Validator interface {
	Validate(token string) (uid string, username string, roles string, exp int64, err error)
	ParseRoles(roles string) map[string]struct{}
}
