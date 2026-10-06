// Package urlhandler joins object keys onto a public base URL and extracts
// keys back from such URLs. It is the shared storage.URLHandler
// implementation embedded by the s3, qiniu, and fileserver drivers; use it
// directly only when building a new driver or mapping keys to URLs without one.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/storage/urlhandler"
//
//	h, err := urlhandler.NewHandler("https://cdn.example.com/assets")
//	if err != nil {
//		return err
//	}
//	u := h.GenerateURL("avatars/a.png")
//	// u == "https://cdn.example.com/assets/avatars/a.png"
//	key := h.ExtractKeyFromURL(u)
//	// key == "avatars/a.png"
//
// # Rules
//
//   - GenerateURL ignores the params argument that storage.URLHandler allows.
//   - Keys that already start with http:// or https:// are returned unchanged
//     by GenerateURL.
//   - ExtractKeyFromURL is strict: the host and base path must match the
//     public base. ExtractKeyFromURLWithMode(uri, false) is lenient.
//   - Keys containing a ".." path segment are refused in both directions:
//     drivers reject them, so such a key could never address an object.
package urlhandler
