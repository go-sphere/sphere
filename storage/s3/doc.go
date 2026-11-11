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
//	// The client PUTs the bytes to auth.Authorization.Value with the headers
//	// in auth.Authorization.Headers; persist auth.File.Key and show
//	// auth.File.URL.
//
// # Behavior
//
//   - Empty PublicBase is derived as http(s)://Endpoint/Bucket; an explicit
//     one must be an absolute http(s) URL.
//   - Content-Type on upload comes from the key's extension.
//   - UploadFile sends a body of up to Config.PartSize with one PutObject and
//     a longer one as a multipart upload, holding at most one part in memory.
//   - ListFiles uses StartAfter; the cursor is the last key of the page. Keys
//     not in storage.NormalizeKey form (such as folder markers "a/") are
//     skipped.
//   - CopyFile stats the source first and copies a source above 5 GiB with a
//     multipart copy. CopyFile and MoveFile with overwrite false check the
//     destination with a separate stat, so the guard is best-effort under
//     concurrent writers. MoveFile is copy then delete and is not atomic;
//     moving or copying onto itself makes no copy.
//   - Presigned URLs live for Config.UploadTTL (default 1 hour);
//     UploadAuthRequest.TTL may only shorten that. They sign a Content-Type
//     from the file name's extension, returned in Authorization.Headers;
//     HTML, SVG, XML, script and other active types are signed as
//     application/octet-stream.
package s3
