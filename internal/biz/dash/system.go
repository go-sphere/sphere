package dash

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/models"
	"strconv"
)

// UploadToken
// @Summary 获取上传凭证
// @Tags dashboard
// @Accept json
// @Produce json
// @Param filename query string true "文件名"
// @Security ApiKeyAuth
// @Success 200 {object} models.UploadToken
// @Router /api/upload/token [get]
func (w *Web) UploadToken(ctx *gin.Context) (gin.H, error) {
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
	return gin.H{
		"token": w.cdn.UploadToken(req.Filename, "dash", cdn.DefaultKeyBuilder(strconv.Itoa(id))),
	}, nil
}

// ResetCache
// @Summary 重置缓存
// @Tags dashboard
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} models.MessageResponse
// @Router /api/cache/reset [post]
func (w *Web) ResetCache(ctx *gin.Context) (*models.MessageResponse, error) {
	err := w.cache.DelAll(ctx)
	if err != nil {
		return nil, err
	}
	return models.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(api gin.IRouter) {
	api.GET("/api/upload/token", web.WithJson(w.UploadToken))
	api.POST("/api/cache/reset", web.WithJson(w.ResetCache))
}
