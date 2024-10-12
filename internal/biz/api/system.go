package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/storage"
	"github.com/tbxark/go-base-api/pkg/web"
	"strconv"
)

// UploadToken
// @Summary 获取上传凭证
// @Tags api
// @Accept json
// @Produce json
// @Param filename query string true "文件名"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[storage.FileUploadToken]
// @Router /api/upload/token [get]
func (w *Web) UploadToken(ctx *gin.Context) (*storage.FileUploadToken, error) {
	var req struct {
		Filename string `form:"filename"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := w.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token := w.Storage.GenerateUploadToken(req.Filename, "user", storage.DefaultKeyBuilder(strconv.Itoa(id)))
	return &token, nil
}

// Status
// @Summary 获取系统状态
// @Tags api
// @Accept json
// @Produce json
// @Success 200 {object} web.MessageResponse
// @Router /api/status [get]
func (w *Web) Status(ctx *gin.Context) (*web.SimpleMessage, error) {
	return web.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(r gin.IRouter) {
	route := r.Group("/")
	route.GET("/api/status", web.WithJson(w.Status))
	route.GET("/api/upload/token", web.WithJson(w.UploadToken))
}
