package safe

import (
	"github.com/go-sphere/sphere/log"
)

func Recover(onError ...func(err any)) {
	if r := recover(); r != nil {
		log.Error(
			"goroutine panic",
			log.String("module", "safe"),
			log.Any("error", r),
		)
		for _, fn := range onError {
			fn(r)
		}
	}
}

func Go(fn func()) {
	go Run(fn)
}

func Run(fn func()) {
	defer Recover()
	fn()
}
