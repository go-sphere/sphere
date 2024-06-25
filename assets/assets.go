package assets

import "embed"

//go:embed dashboard/dist
var DashAssets embed.FS

var DashAssetsPath = "dashboard/dist"
