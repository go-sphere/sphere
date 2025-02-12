package ginx

import "path"

func lastChar(s string) byte {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func JoinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}
	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}
