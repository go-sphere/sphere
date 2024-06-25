package safe

import (
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
)

func Go(id string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorw(
					"goroutine panic",
					field.String("module", "safe"),
					field.String("id", id),
					field.Any("error", r),
				)
			}
		}()
		fn()
	}()
}
