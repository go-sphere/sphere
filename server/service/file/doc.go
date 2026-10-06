// Package file runs a standalone file upload/download HTTP service as a
// core/task.Task. It serves a storage/fileserver.FileServer on a dedicated
// httpx engine; it is not an S3 API.
//
// [NewLocalFileService] builds a FileServer over a local directory, and
// [NewWebServer] wraps it with an engine. When started, the service handles
// PUT /:key (upload with a one-time token from FileServer.GenerateUploadAuth)
// and GET /*filename (download) at the engine root.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/core/boot"
//		"github.com/go-sphere/sphere/server/service/file"
//	)
//
//	files, err := file.NewLocalFileService(file.LocalFileServiceConfig{
//		RootDir:    "./data/files",
//		PublicBase: "http://localhost:9000/",
//	})
//	if err != nil {
//		return nil, err
//	}
//	web := file.NewWebServer(stdx.New(stdx.WithAddr(":9000")), files)
//	return boot.NewApplication(web, apiServer), nil
//
// API handlers then call files.GenerateUploadAuth to hand clients a one-time
// PUT URL, and files.GenerateURL to build download URLs. Web.Stop stops the
// engine and closes the token cache NewLocalFileService created. The service
// does not configure CORS; add it to the engine yourself when browsers upload
// directly.
package file
