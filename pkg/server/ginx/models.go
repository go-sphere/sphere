package ginx

type DataResponse[T any] struct {
	Success bool `json:"success,omitempty" default:"true"`
	Data    T    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success,omitempty" default:"false"`
	Message string `json:"message,omitempty"`
}
