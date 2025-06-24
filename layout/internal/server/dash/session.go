package dash

import (
	"github.com/TBXark/sphere/layout/internal/service/dash"
	"github.com/gin-gonic/gin"
)

func NewSessionMetaData() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(dash.AuthContextKeyIP, ctx.ClientIP())
		ctx.Set(dash.AuthContextKeyUA, ctx.GetHeader("User-Agent"))
		ctx.Next()
	}
}
