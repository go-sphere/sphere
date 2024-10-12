package cdnmodels

type UploadToken struct {
	Token string `json:"token"`
	Key   string `json:"key"`
	URL   string `json:"url"`
}

type UploadResult struct {
	Key string `json:"key"`
}
