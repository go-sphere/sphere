package s3_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/s3"
)

// ExampleNewClient shows construction and presigned upload authorization.
// It needs a reachable S3-compatible endpoint, so it is compiled but not run.
func ExampleNewClient() {
	client, err := s3.NewClient(s3.Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "minio",
		SecretAccessKey: "minio123",
		Bucket:          "assets",
		Dir:             "uploads",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	var _ storage.CDNStorage = client

	auth, err := client.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{
		FileName: "avatar.png",
		Dir:      "avatars",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	// The browser PUTs the file to auth.Authorization.Value; the service
	// persists auth.File.Key and renders auth.File.URL.
	fmt.Println(auth.Authorization.Method, auth.File.Key, auth.File.URL)
}
