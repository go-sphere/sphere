package dash

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
// @Tags dashboard
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
	token := w.cdn.UploadToken(req.Filename, "dash", cdn.DefaultKeyBuilder(strconv.Itoa(id)))
	return &token, nil
}

// CacheReset
// @Summary 重置缓存
// @Tags dashboard
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[models.MessageResponse]
// @Router /api/cache/reset [post]
func (w *Web) CacheReset(ctx *gin.Context) (*models.MessageResponse, error) {
	err := w.cache.DelAll(ctx)
	if err != nil {
		return nil, err
	}
	return models.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(api gin.IRouter) {
	api.GET("/api/upload/token", web.WithJson(w.UploadToken))
	api.POST("/api/cache/reset", web.WithJson(w.CacheReset))
}
