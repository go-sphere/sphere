package safe

import (
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
)

func Go(id string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorw(
					"goroutine panic",
					logfields.String("module", "safe"),
					logfields.String("id", id),
					logfields.Any("error", r),
				)
			}
		}()
		fn()
	}()
}
