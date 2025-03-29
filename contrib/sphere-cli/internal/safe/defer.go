package safe

import "log"

func ErrorIfPresent(label string, fn func() error) {
	err := fn()
	if err != nil {
		log.Printf("%s: %v", label, err)
	}
}
