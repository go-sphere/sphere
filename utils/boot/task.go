package boot

import "context"

type Task interface {
	Identifier() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
