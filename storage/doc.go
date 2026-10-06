// Package storage is the object-storage contract used by Sphere services:
// upload, download, delete, move, and copy, plus optional stat/list and CDN
// URL / upload-authorization capabilities.
//
// Program against [Storage] (or [CDNStorage] when clients upload directly or
// need public URLs) and pick a driver at wiring time:
//
//   - storage/local: files under a root directory (development, single host)
//   - storage/s3: S3-compatible object storage via minio-go ([CDNStorage])
//   - storage/qiniu: Qiniu Kodo ([CDNStorage])
//   - storage/kvcache: blobs kept in a cache.ByteCache
//   - storage/fileserver: an HTTP adapter that turns any Storage into a
//     [CDNStorage] with one-time PUT upload URLs; it is not an S3 driver
//
// # Usage
//
//	import (
//		"context"
//		"io"
//		"strings"
//
//		"github.com/go-sphere/sphere/storage"
//		"github.com/go-sphere/sphere/storage/local"
//	)
//
//	client, err := local.NewClient(local.Config{RootDir: "./data"})
//	if err != nil {
//		return err
//	}
//	var store storage.Storage = client
//	key, err := store.UploadFile(ctx, strings.NewReader("hello"), "/docs/hello.txt")
//	if err != nil {
//		return err
//	}
//	// key is "docs/hello.txt": persist the returned key, not the argument.
//	res, err := store.DownloadFile(ctx, key)
//	if err != nil {
//		return err
//	}
//	defer res.Reader.Close() // the caller owns DownloadResult.Reader
//	body, err := io.ReadAll(res.Reader)
//
// Drivers hold no resources that need closing; storage/fileserver's
// FileServer.Close only matters when it was given an owned cache.
//
// # Keys
//
// Every driver normalizes keys with [NormalizeKey] before use, so a key stored
// by one backend addresses the same object on another. UploadFile returns the
// normalized key. Keys containing a ".." segment, or keys that normalize to
// nothing, fail with storageerr.ErrFileNameInvalid. Use [BuildUploadFileName]
// and [JoinUploadKey] to derive keys from client-supplied file names.
//
// # Errors
//
// Drivers report the sentinels in storage/storageerr; compare with errors.Is:
//
//   - DownloadFile, StatFile, and the source of MoveFile/CopyFile report
//     storageerr.ErrNotFound for a missing key.
//   - MoveFile/CopyFile with overwrite false report storageerr.ErrDestExists
//     when the destination exists.
//   - IsFileExists reports (false, nil) for a missing key.
//   - DeleteFile is idempotent: deleting a missing key succeeds.
//
// # Optional capabilities
//
// [FileStater] and [FileLister] are not part of [Storage]; probe them with a
// type assertion. local, s3, and qiniu implement both; kvcache and fileserver
// implement neither. [UploadAuthRequest].TTL is a ceiling: a client cannot
// extend the driver's configured credential lifetime. [URLHandler].GenerateURL
// produces public URLs; private-bucket signed download URLs are out of scope.
package storage
