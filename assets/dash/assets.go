package dash

import "embed"

//go:embed dashboard/apps/web-ele/dist
var Assets embed.FS

var AssetsPath = "dashboard/apps/web-ele/dist"
