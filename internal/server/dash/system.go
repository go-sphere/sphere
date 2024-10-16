package dash

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/web"
	"strconv"
)

// UploadToken
// @Summary 获取上传凭证
// @Security ApiKeyAuth
// @Param filename query string true "文件名"
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
	token := w.Storage.GenerateUploadToken(req.Filename, "dash", storage.DefaultKeyBuilder(strconv.Itoa(id)))
	return &token, nil
}

// CacheReset
// @Summary 重置缓存
// @Security ApiKeyAuth
// @Success 200 {object} web.MessageResponse
// @Router /api/cache/reset [post]
func (w *Web) CacheReset(ctx *gin.Context) (*web.SimpleMessage, error) {
	err := w.Cache.DelAll(ctx)
	if err != nil {
		return nil, err
	}
	return web.NewSuccessResponse(), nil
}

func (w *Web) bindSystemRoute(r gin.IRouter) {
	route := r.Group("/")
	route.GET("/api/upload/token", web.WithJson(w.UploadToken))
	route.POST("/api/cache/reset", web.WithJson(w.CacheReset))
}
