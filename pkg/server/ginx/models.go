package ginx

import "github.com/gin-gonic/gin"

type DataResponse[T any] struct {
	Success bool `json:"success,omitempty" default:"true"`
	Data    T    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success,omitempty" default:"false"`
	Message string `json:"message,omitempty"`
}

type OperationMiddlewares struct {
	Operations  []string
	Middlewares []gin.HandlerFunc
}

func NewOperationMiddlewares(operation string, middlewares ...gin.HandlerFunc) OperationMiddlewares {
	return OperationMiddlewares{
		Operations:  []string{operation},
		Middlewares: middlewares,
	}
}

func (o OperationMiddlewares) Match(opt string) bool {
	for _, operation := range o.Operations {
		if operation == opt {
			return true
		}
	}
	return false
}

func MatchOperationMiddlewares(oms []OperationMiddlewares, opt string) gin.HandlerFunc {
	fns := make([]gin.HandlerFunc, 0)
	for _, om := range oms {
		if om.Match(opt) {
			fns = append(fns, om.Middlewares...)
		}
	}
	return MiddlewaresGroup(fns...)
}
