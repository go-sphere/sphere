package ginx

type DataResponse[T any] struct {
	Success bool   `json:"success,omitempty"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}
