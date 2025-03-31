package dash

import (
	"encoding/json"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/middleware/auth"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/utils/safe"
	"github.com/gin-gonic/gin"
)

func (w *Web) UploadFile(ctx *gin.Context) (gin.H, error) {
	file, err := ctx.FormFile("file")
	if err != nil {
		return nil, err
	}
	read, err := file.Open()
	if err != nil {
		return nil, err
	}
	filename := storage.DefaultKeyBuilder("dash")(file.Filename, "upload")
	result, err := w.storage.UploadFile(ctx, read, file.Size, filename)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"key": result,
		"url": w.storage.GenerateURL(result),
	}, nil
}

func (w *Web) GetFile(ctx *gin.Context) {
	param := ctx.Param("filename")
	reader, mime, size, err := w.storage.DownloadFile(ctx, param)
	if err != nil {
		ctx.String(404, err.Error())
		return
	}
	defer safe.IfErrorPresent("close reader", reader.Close)
	ctx.DataFromReader(200, size, mime, reader, map[string]string{
		"Cache-Control": "max-age=3600",
	})
}

type UploadFileHandler interface {
	UploadFile(ctx *gin.Context) (gin.H, error)
	GetFile(ctx *gin.Context)
}

func RegisterFileService(route gin.IRouter, handler UploadFileHandler, authParser authorizer.Parser[authorizer.RBACClaims[int64]]) {
	cookieAuth := auth.NewCookieAuthMiddleware("authorized-token", func(raw string) (string, error) {
		var token struct {
			AccessToken string `json:"accessToken"`
		}
		err := json.Unmarshal([]byte(raw), &token)
		if err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}, authParser, true)

	fileRoute := route.Group("/")
	fileRoute.POST("/api/file/upload", ginx.WithJson(handler.UploadFile))
	fileRoute.GET("/api/file/preview/*filename", cookieAuth, handler.GetFile)
}
