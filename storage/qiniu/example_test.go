package qiniu_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/storage"
	"github.com/go-sphere/sphere/storage/qiniu"
)

// ExampleNewClient shows construction and upload-token authorization. Tokens
// are signed locally, so this runs offline; real credentials and a bucket are
// needed for Qiniu to accept the token.
func ExampleNewClient() {
	client, err := qiniu.NewClient(qiniu.Config{
		AccessKey:    "ak",
		SecretKey:    "sk",
		Bucket:       "assets",
		Dir:          "uploads",
		UploadNaming: storage.UploadNamingStrategyOriginal,
		PublicBase:   "https://cdn.example.com",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	var _ storage.CDNStorage = client

	auth, err := client.GenerateUploadAuth(context.Background(), storage.UploadAuthRequest{FileName: "avatar.png"})
	if err != nil {
		fmt.Println(err)
		return
	}
	// The client posts the file to Qiniu's form-upload API with the token in
	// auth.Authorization.Value and the key in auth.File.Key.
	fmt.Println(auth.Authorization.Type, auth.File.Key)
	fmt.Println(client.GenerateImageURL(auth.File.Key, 200))
	// Output:
	// token uploads/avatar.png
	// https://cdn.example.com/uploads/avatar.png?imageView2/2/w/200/q/75
}
