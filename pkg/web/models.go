package web

type PaginationParams struct {
	Page  int `json:"page" form:"page"`
	Limit int `json:"limit" form:"limit"`
}

type IDRequest struct {
	ID int `json:"id" form:"id"`
}

type SimpleMessage struct {
	Message string `json:"message"`
}

func NewSuccessResponse() *SimpleMessage {
	return &SimpleMessage{
		Message: "success",
	}
}

type DataResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type MessageResponse = DataResponse[SimpleMessage]
