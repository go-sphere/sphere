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

	"github.com/go-sphere/sphere/storage/storageerr"
	"github.com/google/uuid"
)

// ResolveUploadTTL picks the validity window for a generated upload
// authorization. configTTL (or defaultTTL when configTTL is unset) acts as a
// ceiling rather than a mere fallback: a request may ask for a shorter-lived
// credential but can never extend one. UploadAuthRequest.TTL is therefore safe
// to populate from client-supplied input, which cannot widen the window a
// deployment configured.
func ResolveUploadTTL(reqTTL, configTTL, defaultTTL time.Duration) time.Duration {
	ceiling := configTTL
	if ceiling <= 0 {
		ceiling = defaultTTL
	}
	if reqTTL > 0 && reqTTL < ceiling {
		return reqTTL
	}
	return ceiling
}

// DefaultKeyBuilder creates a key builder function that generates unique file keys.
// It combines timestamp, MD5 hash of the filename, and preserves the file extension.
// The prefix is prepended to the generated key if provided.
// Format: [prefix_]timestamp_md5hash.ext
//
// Deprecated: unused within the module and unsafe for concurrent uploads of the
// same file name within the same second (second-granularity timestamp + a
// deterministic MD5 of the name collide). Use BuildUploadFileName together with
// JoinUploadKey instead.
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
//
// Deprecated: unused within the module and unsafe for concurrent uploads of the
// same file name within the same second (second-granularity timestamp + a
// deterministic MD5 of the name collide). Use BuildUploadFileName together with
// JoinUploadKey instead.
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

// NormalizeKey returns the canonical form of an object key, or an error when the
// key cannot address an object.
//
// Every driver applies this at its entry points, and UploadFile returns the
// normalized key, so a key persisted by one backend addresses the same object on
// another. Without a shared rule the drivers disagreed: the local driver folded
// "a/" and "d//x" through filepath.Clean but returned the caller's original
// string from UploadFile, so a key stored in a database resolved on local — which
// folded it again on the way back in — and 404'd on s3 or kvcache, which keep
// keys verbatim. An empty key was rejected by local while kvcache stored an
// object reachable only by the empty string.
//
// The rules are:
//   - a leading "/" is dropped, so "/a" and "a" are the same object
//   - repeated separators collapse and "." segments are removed
//   - a trailing "/" is removed
//   - a ".." segment is rejected rather than resolved, so a key can never
//     traverse out of its prefix
//   - a key that normalizes to nothing is rejected
func NormalizeKey(key string) (string, error) {
	trimmed := strings.TrimPrefix(key, "/")
	if trimmed == "" {
		return "", storageerr.ErrFileNameInvalid
	}
	// Checked before cleaning: path.Clean resolves ".." against the preceding
	// segment, which would silently turn a traversal attempt into a valid key
	// somewhere else rather than refusing it.
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", storageerr.ErrFileNameInvalid
		}
	}
	cleaned := strings.TrimPrefix(path.Clean(trimmed), "/")
	if cleaned == "" || cleaned == "." {
		return "", storageerr.ErrFileNameInvalid
	}
	return cleaned, nil
}
