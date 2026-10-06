// Package qiniu is a storage.CDNStorage for Qiniu Kodo built on the official
// go-sdk v7.
//
// [NewClient] returns a [*Client] that implements storage.CDNStorage,
// storage.FileStater, and storage.FileLister. Direct client uploads use Kodo
// upload tokens from GenerateUploadAuth (authorization type "token", sent to
// Kodo's form-upload API); public URLs come from Config.PublicBase.
// [Client.GenerateImageURL] adds Qiniu image resizing. There is no Close.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/storage"
//		"github.com/go-sphere/sphere/storage/qiniu"
//	)
//
//	client, err := qiniu.NewClient(qiniu.Config{
//		AccessKey:  "ak",
//		SecretKey:  "sk",
//		Bucket:     "assets",
//		Dir:        "uploads",
//		PublicBase: "https://cdn.example.com",
//	})
//	if err != nil {
//		return err
//	}
//	auth, err := client.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "avatar.png"})
//	if err != nil {
//		return err
//	}
//	// Hand auth.Authorization.Value (the upload token) and auth.File.Key to
//	// the client; persist auth.File.Key.
//
// # Behavior
//
//   - Upload tokens are insert-only (they cannot replace an object) and
//     scoped to the generated key. Config.MimeLimit restricts the declared
//     Content-Type of token uploads (default DefaultMimeLimit, images and
//     video); Qiniu does not sniff bytes. Server-side UploadFile is not
//     restricted and overwrites.
//   - Token lifetime is Config.UploadTTL (default 1 hour), rounded up to whole
//     seconds; UploadAuthRequest.TTL may only shorten it.
//   - DeleteFile of a missing key succeeds.
//   - DownloadFile reports Size from the download response, falling back to a
//     stat only when the length or content type is unknown; a body that ends
//     short of Size reads as an error wrapping io.ErrUnexpectedEOF.
//   - StatFile, IsFileExists, DeleteFile, MoveFile, and CopyFile ignore ctx:
//     the SDK methods they use have no context variant. Uploads, downloads,
//     and listings honor cancellation.
package qiniu
