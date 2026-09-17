package boot

import (
	"fmt"
	"os"
	"time"
)

// DefaultTimezone is the IANA zone init() installs as time.Local and TZ.
const DefaultTimezone = "Asia/Shanghai"

var versionPrinter = func(version string) {
	fmt.Println(version)
}

func init() {
	if err := InitTimezone(DefaultTimezone); err != nil {
		// tzdata-less environments (scratch/distroless images) fail the zone
		// lookup and silently keep the host default otherwise; say so once.
		// Embed time/tzdata or set the zone explicitly to silence this.
		fmt.Fprintf(os.Stderr, "boot: cannot load timezone %s, keeping host default: %v\n", DefaultTimezone, err)
	}
}

// InitTimezone loads zone, assigns it to time.Local, and sets TZ. Package init
// already calls it with DefaultTimezone; call it again before Run to override.
// A lookup failure leaves time.Local unchanged and returns the error.
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
