package authorizer

import (
	"context"

	"golang.org/x/exp/constraints"
)

// UID represents valid user identifier types: integers or strings. These mirror
// the identifier types used for database primary keys, which are effectively only
// integer or string. For IDs that are neither (for example uuid.UUID), use the
// string form via its String() representation.
type UID interface {
	constraints.Integer | ~string
}

// Claims represents the interface for extracting user information from authentication tokens.
// Implementations should provide methods to extract user ID, subject, and roles.
type Claims[T UID] interface {
	GetUID() (T, error)
	GetSubject() (string, error)
	GetRoles() ([]string, error)
}

// Parser defines the interface for parsing authentication tokens into claims.
type Parser[I UID, T Claims[I]] interface {
	ParseToken(ctx context.Context, token string) (T, error)
}

// Generator defines the interface for generating authentication tokens from claims.
type Generator[I UID, T Claims[I]] interface {
	GenerateToken(ctx context.Context, claims T) (string, error)
}

// TokenAuthorizer combines token parsing and generation capabilities.
type TokenAuthorizer[I UID, T Claims[I]] interface {
	Parser[I, T]
	Generator[I, T]
}

// ParserFunc is a function type that implements the Parser interface.
// This allows functions to be used directly as parsers without defining new types.
type ParserFunc[I UID, T Claims[I]] func(ctx context.Context, token string) (T, error)

// ParseToken implements the Parser interface for ParserFunc.
func (f ParserFunc[I, T]) ParseToken(ctx context.Context, token string) (T, error) {
	return f(ctx, token)
}
