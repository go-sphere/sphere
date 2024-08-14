package telegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tbxark/go-base-api/pkg/cdn/model"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
)

type Telegraph struct {
	endpoint string
	client   *http.Client
}

type Result struct {
	Src string `json:"src"`
}

func NewTelegraph(client *http.Client) *Telegraph {
	if client == nil {
		client = http.DefaultClient
	}
	return &Telegraph{
		endpoint: "https://telegra.ph",
		client:   client,
	}
}

func (t *Telegraph) UploadFile(ctx context.Context, file io.Reader, size int64, key string) (*model.UploadResult, error) {
	if key == "" {
		key = "blob"
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, key))
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", t.endpoint+"/upload", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s:%d", resp.Status, resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	//{"error":"File type invalid"}
	if respBody[0] == '{' {
		var errBody struct {
			Error string `json:"error"`
		}
		err = json.Unmarshal(respBody, &errBody)
		if err != nil {
			return nil, err
		}
		if errBody.Error != "" {
			return nil, fmt.Errorf(errBody.Error)
		}
		return nil, fmt.Errorf("unknown error")
	}
	var result []Result
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	return &model.UploadResult{Key: t.endpoint + result[0].Src}, nil
}

func (t *Telegraph) UploadLocalFile(ctx context.Context, file string, key string) (*model.UploadResult, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	status, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return t.UploadFile(ctx, f, status.Size(), key)
}
