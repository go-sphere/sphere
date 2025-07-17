//go:build embed_dash

package dash

import "embed"

// IMPORTANT:
// All files in the subtree rooted at that directory are embedded (recursively), except that files with names beginning with ‘.’ or ‘_’ are excluded.

// You can `git clone https://github.com/pure-admin/vue-pure-admin.git $(DASH_DIR)` to get the dash project

//go:embed dashboard/dist
var Assets embed.FS

var AssetsPath = "dashboard/dist"
