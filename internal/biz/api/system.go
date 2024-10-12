package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/cdn/cdnmodels"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/webmodels"
	"strconv"
)

// UploadToken
// @Summary 获取上传凭证
// @Tags api
// @Accept json
// @Produce json
// @Param filename query string true "文件名"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[cdnmodels.UploadToken]
// @Router /api/upload/token [get]
func (w *Web) UploadToken(ctx *gin.Context) (*cdnmodels.UploadToken, error) {
	var req struct {
		Filename string `form:"filename"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		return nil, err
	}
	if req.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	id, err := w.auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	token := w.cdn.UploadToken(req.Filename, "user", cdn.DefaultKeyBuilder(strconv.Itoa(id)))
	return &token, nil
}

// Status
// @Summary 获取系统状态
// @Tags api
// @Accept json
// @Produce json
// @Success 200 {object} MessageResponse
// @Router /api/status [get]
func (w *Web) Status(ctx *gin.Context) (*webmodels.MessageResponse, error) {
	return webmodels.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(r gin.IRouter) {
	r.GET("/api/status", web.WithJson(w.Status))
	r.GET("/api/upload/token", web.WithJson(w.UploadToken))
}
