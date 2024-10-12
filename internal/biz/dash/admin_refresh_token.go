package dash

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/pkg/web/auth/jwtauth"
	"github.com/tbxark/go-base-api/pkg/web/webmodels"
	"strconv"
)

type AdminRefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type AdminRefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Expires      string `json:"expires"`
}

// AdminRefreshToken
// @Summary 刷新管理员Token
// @Tags dashboard
// @Accept json
// @Produce json
// @Param login body AdminRefreshTokenRequest true "刷新信息"
// @Success 200 {object} web.DataResponse[AdminLoginResponse]
// @Router /api/admin/refresh-token [post]
func (w *Web) AdminRefreshToken(ctx *gin.Context) (*AdminLoginResponse, error) {
	var body AdminRefreshTokenRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		return nil, err
	}
	claims, err := w.JwtAuth.Validate(body.RefreshToken)
	if err != nil {
		return nil, err
	}
	uid, ok := claims[jwtauth.SignedDetailsUidKey].(string)
	if !ok {
		return nil, webmodels.NewHTTPError(400, "uid not found")
	}
	id, err := strconv.Atoi(uid)
	if err != nil {
		return nil, err
	}
	u, err := w.DB.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return w.createLoginResponse(u)
}
