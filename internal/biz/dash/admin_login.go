package dash

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/dao/ent/admin"
	"github.com/tbxark/go-base-api/pkg/web/webmodels"
	"strconv"
	"time"
)

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AdminLoginResponse struct {
	Avatar       string   `json:"avatar"`
	Username     string   `json:"username"`
	Nickname     string   `json:"nickname"`
	Roles        []string `json:"roles"`
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	Expires      string   `json:"expires"`
}

func (w *Web) createLoginResponse(u *ent.Admin) (*AdminLoginResponse, error) {
	id := strconv.Itoa(u.ID)
	token, err := w.JwtAuth.GenerateSignedToken(id, u.Username, u.Roles...)
	if err != nil {
		return nil, err
	}
	refresh, err := w.JwtAuth.GenerateRefreshToken(id)
	if err != nil {
		return nil, err
	}
	return &AdminLoginResponse{
		Avatar:       w.CDN.RenderImageURL(u.Avatar, 512),
		Username:     u.Username,
		Nickname:     u.Nickname,
		Roles:        u.Roles,
		AccessToken:  token.Token,
		RefreshToken: refresh.Token,
		Expires:      token.ExpiresAt.Format(time.DateTime),
	}, nil
}

// AdminLogin
// @Summary 管理员登录
// @Tags dashboard
// @Accept json
// @Produce json
// @Param login body AdminLoginRequest true "登录信息"
// @Success 200 {object} web.DataResponse[AdminLoginResponse]
// @Router /api/admin/login [post]
func (w *Web) AdminLogin(ctx *gin.Context) (*AdminLoginResponse, error) {
	var req AdminLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	u, err := w.DB.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if !encrypt.IsPasswordMatch(req.Password, u.Password) {
		return nil, webmodels.NewHTTPError(400, "password not match")
	}
	return w.createLoginResponse(u)
}
