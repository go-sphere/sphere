package bot

import (
	"testing"
)

func TestApp_Run(t *testing.T) {
	app := NewApp(&Config{
		Token: "7345358070:AAEDBxbC0Tjs5CxFroPAqIJ3-rENi4Pj61E",
	})
	app.Run()
}
