package zip

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func downloadZipReader(url string) (*zip.Reader, func(), error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New(resp.Status)
	}
	tempFile, err := os.CreateTemp("", "zip-*")
	if err != nil {
		return nil, nil, err
	}
	length, err := io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, nil, err
	}
	reader, err := zip.NewReader(tempFile, length)
	if err != nil {
		return nil, nil, err
	}
	return reader, func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}, nil
}

func ensureSafePath(tempDir, fileName string) (string, error) {
	filePath := filepath.Join(tempDir, fileName)
	if !strings.HasPrefix(filePath, filepath.Clean(tempDir)) {
		return "", fmt.Errorf("unsafe file path: %s", filePath)
	}
	return filePath, nil
}

func unzipFile(file *zip.File, tempDir string) error {
	filePath, err := ensureSafePath(tempDir, file.Name)
	if err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(filePath, 0o755)
	}
	err = os.MkdirAll(filepath.Dir(filePath), 0o755)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dstFile.Close()
	}()

	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func DownloadAndUnzip(url string) (string, error) {
	zipReader, clean, err := downloadZipReader(url)
	if err != nil {
		return "", err
	}
	defer clean()

	tempDir, err := os.MkdirTemp("", "unzip-*")
	if err != nil {
		return "", err
	}
	for _, file := range zipReader.File {
		if zErr := unzipFile(file, tempDir); zErr != nil {
			return "", zErr
		}
	}
	return tempDir, nil
}
