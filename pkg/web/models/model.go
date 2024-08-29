package models

type PageQuery struct {
	Page  int `json:"page" form:"page"`
	Limit int `json:"limit" form:"limit"`
}

type IDRequest struct {
	ID int `json:"id" form:"id"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

func NewSuccessResponse() *MessageResponse {
	return &MessageResponse{
		Message: "success",
	}
}
