// Package fileserver turns any storage.Storage into a storage.CDNStorage
// served over HTTP: public download URLs under GetBase and one-time PUT
// upload URLs under PutBase. It is an HTTP adapter, not an S3 driver.
//
// [NewCDNAdapter] wraps a store and a cache.ByteCache that holds one-time
// upload tokens (token → key, redeemed with GetDel). Mount
// [FileServer.RegisterFileUploader] at PutBase and
// [FileServer.RegisterFileDownloader] at GetBase on any httpx router, then
// hand clients the result of [FileServer.GenerateUploadAuth].
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/cache/memory"
//		"github.com/go-sphere/sphere/storage"
//		"github.com/go-sphere/sphere/storage/fileserver"
//		"github.com/go-sphere/sphere/storage/local"
//	)
//
//	store, err := local.NewClient(local.Config{RootDir: "./data"})
//	if err != nil {
//		return err
//	}
//	fs, err := fileserver.NewCDNAdapter(fileserver.Config{
//		PutBase: "https://files.example.com/upload",
//		GetBase: "https://files.example.com/files",
//	}, memory.NewByteCache(), store, fileserver.WithOwnedCache())
//	if err != nil {
//		return err
//	}
//	defer fs.Close() // closes the token cache because of WithOwnedCache
//
//	engine := stdx.New()
//	fs.RegisterFileUploader(engine.Group("/upload"))
//	fs.RegisterFileDownloader(engine.Group("/files"))
//
//	auth, err := fs.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "a.png"})
//	// The client PUTs the raw bytes to auth.Authorization.Value once, then
//	// downloads from auth.File.URL.
//
// # Behavior
//
//   - PutBase and GetBase are required. KeyTTL of 0 becomes 5 minutes and is
//     also the ceiling for UploadAuthRequest.TTL.
//   - A token is spent on its first PUT, even if storing the body fails.
//   - Downloads set X-Content-Type-Options: nosniff and
//     Content-Disposition: attachment unless [WithInlineDownload] is used.
//   - The cache belongs to the caller unless [WithOwnedCache] is given; the
//     store is never closed by FileServer.
//   - FileServer does not implement storage.FileStater or storage.FileLister.
package fileserver
