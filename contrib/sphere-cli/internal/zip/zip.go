package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func UnzipToTemp(uri string) (tempDir string, err error) {
	tempDir, err = os.MkdirTemp("", "unzip_*")
	if err != nil {
		return "", fmt.Errorf("create temp directory failed: %w", err)
	}

	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	zipData, err := downloadFile(uri)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	err = extractZip(zipData, tempDir)
	if err != nil {
		return "", fmt.Errorf("extraction failed: %w", err)
	}

	return tempDir, nil
}

func downloadFile(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer ifErrorPresent("close response body", resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	const maxSize = 100 << 20 // 100MB
	zipData, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if len(zipData) == 0 {
		return nil, fmt.Errorf("empty zip file")
	}

	return zipData, nil
}

func extractZip(zipData []byte, destDir string) error {
	reader, rErr := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if rErr != nil {
		return fmt.Errorf("create zip reader failed: %w", rErr)
	}
	destDir = filepath.Clean(destDir) + string(filepath.Separator)

	for _, file := range reader.File {

		filePath := filepath.Clean(filepath.Join(destDir, file.Name))
		if !strings.HasPrefix(filepath.Clean(filePath), destDir) {
			return fmt.Errorf("illegal file path: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, file.Mode()); err != nil {
				return fmt.Errorf("create directory failed: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
			return fmt.Errorf("create parent directory failed: %w", err)
		}
		if err := extractFile(file, filePath); err != nil {
			return fmt.Errorf("extract file failed: %w", err)
		}
	}

	return nil
}

func extractFile(zipFile *zip.File, destPath string) error {
	srcFile, err := zipFile.Open()
	if err != nil {
		return fmt.Errorf("open zip entry failed: %w", err)
	}
	defer ifErrorPresent("close zip entry", srcFile.Close)

	dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zipFile.Mode())
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer ifErrorPresent("close file", dstFile.Close)

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy file failed: %w", err)
	}

	return nil
}

func ifErrorPresent(label string, fn func() error) {
	err := fn()
	if err != nil {
		log.Printf("%s: %v", label, err)
	}
}
