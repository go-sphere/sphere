package ginx

type DataResponse[T any] struct {
	Code int `json:"code" default:"0"`
	Data T   `json:"data,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code" default:"1"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
