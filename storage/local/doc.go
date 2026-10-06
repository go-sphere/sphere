// Package local is a filesystem storage.Storage rooted at Config.RootDir.
//
// Use it for development, tests, and single-host deployments. [NewClient]
// creates RootDir if needed and returns a [*Client] that implements
// storage.Storage, storage.FileStater, and storage.FileLister. It has no
// URLHandler or UploadAuthorizer; wrap it with storage/fileserver when clients
// need public URLs or direct uploads. There is nothing to close.
//
// # Usage
//
//	import (
//		"context"
//		"strings"
//
//		"github.com/go-sphere/sphere/storage/local"
//	)
//
//	client, err := local.NewClient(local.Config{RootDir: "./data"})
//	if err != nil {
//		return err
//	}
//	key, err := client.UploadFile(ctx, strings.NewReader("hello"), "docs/hello.txt")
//	if err != nil {
//		return err
//	}
//	res, err := client.DownloadFile(ctx, key)
//	if err != nil {
//		return err
//	}
//	defer res.Reader.Close()
//
// # Behavior
//
//   - Writes go to a temporary file in the target directory, then rename and
//     directory fsync, so readers never see a partially written object.
//   - Keys are normalized with storage.NormalizeKey and confined to RootDir
//     lexically; symlinks already inside RootDir are followed, so RootDir is
//     not a symlink-safe jail and its contents must be trusted.
//   - New files are created 0o644 and directories 0o750; overwrites keep the
//     existing file's permissions.
//   - ctx is checked before a write starts but not during the copy.
//   - UploadLocalFile copies the source; it does not move or remove it.
//   - Directories are never treated as objects: they read as missing.
//   - ListFiles walks and sorts every key under the prefix on each page call,
//     so it costs O(matching files) per page; it skips in-progress
//     temporary files and non-regular files.
//   - The no-overwrite check in MoveFile and CopyFile is exact for a single
//     writer and best-effort against concurrent writers.
package local
