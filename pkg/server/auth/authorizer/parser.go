package authorizer

type Claims interface {
	Valid() error
}

type Parser[T Claims] interface {
	ParseToken(token string) (*T, error)
}

type Generator[T Claims] interface {
	GenerateToken(claims *T) (string, error)
}

type TokenAuthorizer[T Claims] interface {
	Parser[T]
	Generator[T]
}
