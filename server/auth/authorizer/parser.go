package authorizer

import "context"

type Parser[T any] interface {
	ParseToken(ctx context.Context, token string) (*T, error)
}

type Generator[T any] interface {
	GenerateToken(ctx context.Context, claims *T) (string, error)
}

type TokenAuthorizer[T any] interface {
	Parser[T]
	Generator[T]
}
