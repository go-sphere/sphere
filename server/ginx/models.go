package ginx

type DataResponse[T any] struct {
	Success bool `json:"succeed" default:"true"`
	Code    int  `json:"code" default:"0"`
	Data    T    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"succeed" default:"false"`
	Code    int    `json:"code" default:"1"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
