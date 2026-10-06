package httpz

import (
	"errors"
	"io/fs"
	"os"
)

// Fs returns a fs.FS from a local directory if it exists, otherwise from
// fs.Sub(files, emPath). It errors when neither source is usable.
//
// local is ignored when empty, missing, or not a directory. The usual pattern
// serves an embedded asset tree in production and lets a developer override it
// with a checked-out directory:
//
//	//go:embed dist
//	var assets embed.FS
//
//	files, err := httpz.Fs("./dist", assets, "dist")
//	if err != nil {
//		return err
//	}
//	router.StaticFS("/", files)
func Fs(local string, files fs.FS, emPath string) (fs.FS, error) {
	// 1. Try the local directory first
	if local != "" {
		info, err := os.Stat(local)
		if err == nil && info.IsDir() {
			return os.DirFS(local), nil
		}
	}
	// 2. Fallback to embedded filesystem
	if files != nil {
		sub, err := fs.Sub(files, emPath)
		if err != nil {
			return nil, err
		}
		return sub, nil
	}
	// 3. Nothing available
	return nil, errors.New("no valid filesystem source: local dir or embedded fs")
}
