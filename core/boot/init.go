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
	_ = InitTimezone(DefaultTimezone)
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
