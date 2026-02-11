package storage

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultKeyBuilder creates a key builder function that generates unique file keys.
// It combines timestamp, MD5 hash of the filename, and preserves the file extension.
// The prefix is prepended to the generated key if provided.
// Format: [prefix_]timestamp_md5hash.ext
func DefaultKeyBuilder(prefix string) func(fileName string, dir ...string) string {
	return func(fileName string, dir ...string) string {
		fileExt := path.Ext(fileName)
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5 + fileExt
		if prefix != "" {
			name = prefix + "_" + name
		}
		return path.Join(path.Join(dir...), name)
	}
}

// KeepFileNameKeyBuilder creates a key builder that preserves the original filename.
// It generates a unique directory path using timestamp and MD5 hash, then stores
// the file with its original name within that directory.
// Format: timestamp_md5hash/original_filename
func KeepFileNameKeyBuilder() func(fileName string, dir ...string) string {
	return func(fileName string, dir ...string) string {
		sum := md5.Sum([]byte(fileName))
		nameMd5 := hex.EncodeToString(sum[:])
		name := strconv.Itoa(int(time.Now().Unix())) + "_" + nameMd5
		return path.Join(path.Join(dir...), name, fileName)
	}
}

// BuildUploadFileName builds the upload file name by strategy.
func BuildUploadFileName(fileName string, strategy UploadNamingStrategy) (string, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", errors.New("file_name is required")
	}
	if strategy == "" {
		strategy = UploadNamingStrategyRandomExt
	}

	fileExt := path.Ext(fileName)
	switch strategy {
	case UploadNamingStrategyRandomExt:
		return uuid.NewString() + fileExt, nil
	case UploadNamingStrategyHashExt:
		sum := md5.Sum([]byte(fileName))
		return hex.EncodeToString(sum[:]) + fileExt, nil
	case UploadNamingStrategyOriginal:
		base := path.Base(fileName)
		if base == "." || base == ".." || base == "/" || strings.TrimSpace(base) == "" {
			return "", errors.New("invalid original file name")
		}
		return base, nil
	default:
		return "", errors.New("unsupported upload naming strategy")
	}
}

// JoinUploadKey joins configured prefix dir, business dir and file name into a safe key.
func JoinUploadKey(prefixDir string, bizDir string, fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "", errors.New("file_name is required")
	}
	if path.IsAbs(fileName) {
		return "", errors.New("file_name must be relative")
	}
	fileName = path.Clean(fileName)
	if fileName == "." || fileName == ".." || strings.HasPrefix(fileName, "../") {
		return "", errors.New("invalid file_name")
	}

	prefix, err := normalizeUploadDir(prefixDir, false, "prefix_dir")
	if err != nil {
		return "", err
	}
	biz, err := normalizeUploadDir(bizDir, true, "biz_dir")
	if err != nil {
		return "", err
	}

	key := path.Join(prefix, biz, fileName)
	if key == "." || key == "" {
		return "", errors.New("invalid upload key")
	}
	if prefix != "" && key != prefix && !strings.HasPrefix(key, prefix+"/") {
		return "", fmt.Errorf("upload key escaped prefix_dir: %q", key)
	}
	return key, nil
}

func normalizeUploadDir(raw string, rejectAbs bool, field string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if rejectAbs && path.IsAbs(value) {
		return "", fmt.Errorf("%s must be relative", field)
	}
	value = strings.TrimPrefix(value, "/")
	value = path.Clean(value)
	if value == "." {
		return "", nil
	}
	if value == ".." || strings.HasPrefix(value, "../") {
		return "", fmt.Errorf("%s must not contain parent path", field)
	}
	return value, nil
}
