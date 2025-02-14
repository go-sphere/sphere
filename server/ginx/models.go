package ginx

type DataResponse[T any] struct {
	Success bool `json:"success" default:"true"`
	Code    int  `json:"code,omitempty" default:"0"`
	Data    T    `json:"data"`
}

type ErrorResponse struct {
	Success bool   `json:"success,omitempty" default:"false"`
	Code    int    `json:"code,omitempty" default:"0"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
