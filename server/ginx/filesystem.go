package ginx

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Fs creates an http.FileSystem from either a local directory or an embedded filesystem.
// It prioritizes the local path if provided, otherwise uses the embedded filesystem.
// Returns nil if neither source is available.
func Fs(local string, emFs *embed.FS, emPath string) (http.FileSystem, error) {
	if local != "" {
		return gin.Dir(local, true), nil
	}
	if emFs != nil {
		sf, err := fs.Sub(emFs, emPath)
		if err != nil {
			return nil, err
		}
		return http.FS(sf), nil
	}
	return nil, nil
}
