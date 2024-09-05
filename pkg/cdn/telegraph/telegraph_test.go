package telegraph

import (
	"context"
	"net/http"
	"testing"
)

func TestTelegraph_UploadFile(t *testing.T) {
	uploader := NewTelegraph(nil)

	imageurl := "https://tbxark.com/assets/avatar.png"
	resp, err := http.Get(imageurl)
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	res, err := uploader.UploadFile(context.TODO(), resp.Body, resp.ContentLength, "avatar.png")
	if err != nil {
		t.Error(err)
	} else {
		t.Log(res.Key)
	}
}
