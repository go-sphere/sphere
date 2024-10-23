package authorizer

import (
	"golang.org/x/exp/constraints"
	"time"
)

type UID interface {
	constraints.Integer | string
}

type Claims[T UID] struct {
	UID     T
	Subject string
	Roles   string
	Exp     int64
}

type Token struct {
	Token     string
	ExpiresAt time.Time
}

type Parser[T UID] interface {
	ParseToken(token string) (*Claims[T], error)
	ParseRoles(roles string) []string
}

type Generator[T UID] interface {
	GenerateToken(uid T, subject string, roles ...string) (*Token, error)
}

type Authorizer[T UID] interface {
	Parser[T]
	Generator[T]
}
