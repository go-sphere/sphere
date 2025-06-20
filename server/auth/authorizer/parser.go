package authorizer

import (
	"context"

	"golang.org/x/exp/constraints"
)

type UID interface {
	constraints.Integer | string
}

type Claims[T UID] interface {
	GetUID() (T, error)
	GetSubject() (string, error)
	GetRoles() ([]string, error)
}

type Parser[I UID, T Claims[I]] interface {
	ParseToken(ctx context.Context, token string) (T, error)
}

type Generator[I UID, T Claims[I]] interface {
	GenerateToken(ctx context.Context, claims T) (string, error)
}

type TokenAuthorizer[I UID, T Claims[I]] interface {
	Parser[I, T]
	Generator[I, T]
}
