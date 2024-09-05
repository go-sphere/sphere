package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/cdn"
	cdnModel "github.com/tbxark/go-base-api/pkg/cdn/models"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/models"
	"strconv"
)

type UploadTokenResponse = cdnModel.UploadToken

// UploadToken
// @Summary 获取上传凭证
// @Tags api
// @Accept json
// @Produce json
// @Param filename query string true "文件名"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[UploadTokenResponse]
// @Router /api/upload/token [get]
func (w *Web) UploadToken(ctx *gin.Context) (*UploadTokenResponse, error) {
	var req struct {
		Filename string `form:"filename"`
	}
	if err := ctx.BindQuery(&req); err != nil {
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
// @Success 200 {object} web.DataResponse[models.MessageResponse]
// @Router /api/status [get]
func (w *Web) Status(ctx *gin.Context) (*models.MessageResponse, error) {
	return models.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(r gin.IRouter) {
	r.GET("/api/status", web.WithJson(w.Status))
	r.GET("/api/upload/token", web.WithJson(w.UploadToken))
}
