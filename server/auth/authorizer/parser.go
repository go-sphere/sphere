package authorizer

type Parser[T any] interface {
	ParseToken(token string) (*T, error)
}

type Generator[T any] interface {
	GenerateToken(claims *T) (string, error)
}

type TokenAuthorizer[T any] interface {
	Parser[T]
	Generator[T]
}
