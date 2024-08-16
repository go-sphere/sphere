package tmpl

import (
	"embed"
)

//go:embed assets/*.tmpl
var Assets embed.FS

const AssetsDir = "assets"
