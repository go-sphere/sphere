package storage

import (
	"crypto/md5"
	"encoding/hex"
	"path"
	"strconv"
	"time"
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
