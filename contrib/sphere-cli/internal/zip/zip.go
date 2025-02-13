package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func UnzipToTemp(uri string) (tempDir string, err error) {
	tempDir, err = os.MkdirTemp("", "unzip_*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
			tempDir = ""
		}
	}()

	resp, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	rawZip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	reader, err := zip.NewReader(bytes.NewReader(rawZip), int64(len(rawZip)))
	if err != nil {
		return "", err
	}

	for _, file := range reader.File {
		path := filepath.Join(tempDir, file.Name)
		if !filepath.IsLocal(file.Name) {
			return "", fmt.Errorf("invalid file name: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if e := os.MkdirAll(path, file.Mode()); e != nil {
				return "", e
			}
			continue
		}

		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return "", e
		}

		dstFile, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if e != nil {
			return "", e
		}

		srcFile, e := file.Open()
		if e != nil {
			_ = dstFile.Close()
			return "", e
		}

		_, e = io.Copy(dstFile, srcFile)
		_ = srcFile.Close()
		_ = dstFile.Close()
		if e != nil {
			return "", e
		}
	}
	return tempDir, nil
}
