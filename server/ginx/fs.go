package ginx

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
