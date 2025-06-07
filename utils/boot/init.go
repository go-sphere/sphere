package boot

import (
	"fmt"
	"os"
	"time"
)

const DefaultTimezone = "Asia/Shanghai"

var versionPrinter = func(version string) {
	fmt.Println(version)
}

func init() {
	_ = InitTimezone(DefaultTimezone)
}

func InitTimezone(zone string) error {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return err
	}
	time.Local = loc
	return os.Setenv("TZ", zone)
}

func InitVersionPrinter(printer func(string)) {
	versionPrinter = printer
}
