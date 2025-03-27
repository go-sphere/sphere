//go:build embed_dash

package dash

import "embed"

//go:embed dashboard/dist
var Assets embed.FS

var AssetsPath = "dashboard/dist"
