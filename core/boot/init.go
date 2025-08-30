package boot

import (
	"fmt"
	"os"
	"time"
)

// DefaultTimezone is the default timezone used for application initialization.
const DefaultTimezone = "Asia/Shanghai"

var versionPrinter = func(version string) {
	fmt.Println(version)
}

func init() {
	_ = InitTimezone(DefaultTimezone)
}

// InitTimezone sets the application timezone to the specified zone.
// It loads the timezone location and sets both time.Local and TZ environment variable.
func InitTimezone(zone string) error {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	time.Local = loc
	return os.Setenv("TZ", zone)
}

// InitVersionPrinter sets a custom version printer function to replace the default.
// This allows applications to customize how version information is displayed.
func InitVersionPrinter(printer func(string)) {
	versionPrinter = printer
}
