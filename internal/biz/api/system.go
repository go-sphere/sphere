package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/web"
	"strconv"
)

// UploadToken
// @Summary 获取上传凭证
// @Tags api
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param filename query string true "文件名"
// @Success 200 {object} cdn.UploadToken
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
		"token": w.cdn.UploadToken(req.Filename, "user", cdn.DefaultKeyBuilder(strconv.Itoa(id))),
	}, nil
}

func (w *Web) bindSystemRoute(r gin.IRouter) {
	r.GET("/api/upload/token", web.WithJson(w.UploadToken))
}
