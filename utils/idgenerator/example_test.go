package idgenerator_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/go-sphere/sphere/utils/idgenerator"
)

// ExampleInitFromEnv shows the boot-time setup of the global generator. It
// changes process-global state, so it is compiled but not run.
func ExampleInitFromEnv() {
	// Call once, early in main; WORKER_ID must be unique per process.
	if err := idgenerator.InitFromEnv(); err != nil && !errors.Is(err, idgenerator.ErrAlreadyInitialized) {
		log.Fatal(err)
	}
	fmt.Println(idgenerator.NextId() > 0)
}

func ExampleNewIdGenerator() {
	next := idgenerator.NewIdGenerator(7)
	a, b := next(), next()
	fmt.Println(a > 0, b > a)
	// Output:
	// true true
}
