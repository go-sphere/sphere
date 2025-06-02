//go:build embed_dash

package dash

import "embed"

// IMPORTANT:
// All files in the subtree rooted at that directory are embedded (recursively), except that files with names beginning with ‘.’ or ‘_’ are excluded.

//go:embed dashboard/dist
var Assets embed.FS

var AssetsPath = "dashboard/dist"
