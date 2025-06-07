package safe

import (
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

func Recover(onError ...func(err any)) {
	if r := recover(); r != nil {
		log.Errorw(
			"goroutine panic",
			logfields.String("module", "safe"),
			logfields.Any("error", r),
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
