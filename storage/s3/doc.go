// Package s3 is a storage.CDNStorage for S3-compatible object storage (AWS
// S3, MinIO, R2, ...) built on minio-go.
//
// [NewClient] returns a [*Client] that implements storage.CDNStorage,
// storage.FileStater, and storage.FileLister. Direct client uploads use
// presigned PUT URLs from GenerateUploadAuth; public URLs come from
// Config.PublicBase. There is no Close; the HTTP transport lives with the
// process.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/storage"
//		"github.com/go-sphere/sphere/storage/s3"
//	)
//
//	client, err := s3.NewClient(s3.Config{
//		Endpoint:        "localhost:9000",
//		AccessKeyID:     "minio",
//		SecretAccessKey: "minio123",
//		Bucket:          "assets",
//		Dir:             "uploads",
//	})
//	if err != nil {
//		return err
//	}
//	auth, err := client.GenerateUploadAuth(ctx, storage.UploadAuthRequest{
//		FileName: "avatar.png",
//		Dir:      "avatars",
//	})
//	if err != nil {
//		return err
//	}
//	// The client PUTs the bytes to auth.Authorization.Value; persist
//	// auth.File.Key and show auth.File.URL.
//
// # Behavior
//
//   - Empty PublicBase is derived as http(s)://Endpoint/Bucket.
//   - Content-Type on upload comes from the key's extension.
//   - ListFiles uses StartAfter; the cursor is the last key of the page.
//   - CopyFile and MoveFile with overwrite false check the destination with a
//     separate stat, so the guard is best-effort under concurrent writers.
//     MoveFile is copy then delete and is not atomic; moving onto itself is a
//     no-op.
//   - Presigned URLs live for Config.UploadTTL (default 1 hour);
//     UploadAuthRequest.TTL may only shorten that.
package s3
