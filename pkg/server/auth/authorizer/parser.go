package authorizer

import "time"

type Claims struct {
	Subject  string
	Username string
	Roles    string
	Exp      int64
}

type Token struct {
	Token     string
	ExpiresAt time.Time
}

type Parser interface {
	ParseToken(token string) (*Claims, error)
	ParseRoles(roles string) []string
}

type Generator interface {
	GenerateToken(subject, username string, roles ...string) (*Token, error)
}

type Authorizer interface {
	Parser
	Generator
}
