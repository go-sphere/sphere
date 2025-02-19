package models

type FileUploadToken struct {
	Token string `json:"token"`
	Key   string `json:"key"`
	URL   string `json:"url"`
}

type FileUploadResult struct {
	Key string `json:"key"`
}
