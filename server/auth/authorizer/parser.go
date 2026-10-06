package authorizer

import (
	"context"
)

// UID represents valid user identifier types: integers or strings. These mirror
// the identifier types used for database primary keys, which are effectively only
// integer or string. For IDs that are neither (for example uuid.UUID), use the
// string form via its String() representation.
//
// The union mirrors what golang.org/x/exp/constraints.Integer unioned with
// ~string would express, inlined so the module does not depend on x/exp for a
// single constraint.
type UID interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~string
}

// Claims represents the interface for extracting user information from authentication tokens.
// Implementations should provide methods to extract user ID, subject, and roles.
//
// An error from GetUID rejects the request, because the identity cannot be defaulted.
// GetSubject and GetRoles are optional: when they fail the field is left at its zero
// value and the request continues, so absent data should be reported as a zero value
// with a nil error rather than as an error.
type Claims[T UID] interface {
	GetUID() (T, error)
	GetSubject() (string, error)
	GetRoles() ([]string, error)
}

// Parser defines the interface for parsing authentication tokens into claims.
//
// ParseToken receives the token with any transport prefix (such as "Bearer ")
// already removed by the caller. Auth middleware treats claims returned with a
// nil error as authenticated, so an implementation must return an error for
// any token it does not trust (malformed, wrongly signed, or expired).
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
