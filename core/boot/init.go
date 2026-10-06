package boot

import (
	"fmt"
	"os"
	"time"
)

// DefaultTimezone is the IANA zone the layout templates pass to InitTimezone.
// The package does not apply it on its own: importing boot leaves time.Local
// and TZ as the host configured them.
const DefaultTimezone = "Asia/Shanghai"

var versionPrinter = func(version string) {
	fmt.Println(version)
}

// InitTimezone loads zone, assigns it to time.Local, and sets TZ. Nothing in
// this package calls it; call it from main, before anything reads time.Local
// (ideally first), for example InitTimezone(DefaultTimezone).
// A lookup failure leaves time.Local unchanged and returns the error. Images
// without tzdata (scratch/distroless) fail the lookup unless the binary embeds
// time/tzdata.
func InitTimezone(zone string) error {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	time.Local = loc
	return os.Setenv("TZ", zone)
}

// InitVersionPrinter replaces the function DefaultConfigParser uses for -version.
func InitVersionPrinter(printer func(string)) {
	versionPrinter = printer
}
