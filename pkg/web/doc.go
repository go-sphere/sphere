package web

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"
	"net/http"
)

func SetupDoc(doc *swag.Spec, title string, route gin.IRoutes) {
	doc.Title = title
	route.GET("/swagger-raw/doc.json", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, doc.ReadDoc())
	})
	route.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("/swagger-raw/doc.json"),
	))
}
