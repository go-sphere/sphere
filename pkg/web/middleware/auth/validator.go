package auth

type Validator interface {
	Validate(token string) (map[string]any, error)
	ParseRolesString(roles string) map[string]struct{}
}
