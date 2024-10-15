package dash

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/internal/pkg/database/ent"
	"github.com/tbxark/go-base-api/internal/pkg/database/ent/admin"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/pkg/web"
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

type AdminLoginResponseWrapper = web.DataResponse[AdminLoginResponse]

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
		Avatar:       w.Storage.GenerateImageURL(u.Avatar, 512),
		Username:     u.Username,
		Nickname:     u.Nickname,
		Roles:        u.Roles,
		AccessToken:  token.Token,
		RefreshToken: refresh.Token,
		Expires:      token.ExpiresAt.Format(time.DateTime),
	}, nil
}

// AuthLogin
//
//	@Summary	管理员登录
//	@Tags		dashboard
//	@Accept		json
//	@Produce	json
//	@Param		login	body		AdminLoginRequest	true	"登录信息"
//	@Success	200		{object}	AdminLoginResponseWrapper
//	@Router		/api/auth/login [post]
func (w *Web) AuthLogin(ctx *gin.Context) (*AdminLoginResponse, error) {
	var req AdminLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	u, err := w.DB.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if !encrypt.IsPasswordMatch(req.Password, u.Password) {
		return nil, web.NewHTTPError(400, "password not match")
	}
	return w.createLoginResponse(u)
}

type AdminRefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// AuthRefresh
//
//	@Summary	刷新管理员Token
//	@Tags		dashboard
//	@Accept		json
//	@Produce	json
//	@Param		login	body		AdminRefreshTokenRequest	true	"刷新信息"
//	@Success	200		{object}	AdminLoginResponseWrapper
//	@Router		/api/auth/refresh [post]
func (w *Web) AuthRefresh(ctx *gin.Context) (*AdminLoginResponse, error) {
	var body AdminRefreshTokenRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		return nil, err
	}
	claims, err := w.JwtAuth.ParseToken(body.RefreshToken)
	if err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return nil, err
	}
	u, err := w.DB.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return w.createLoginResponse(u)
}

func (w *Web) bindAuthRoute(r gin.IRouter) {
	route := r.Group("/")
	route.POST("/api/auth/login", web.WithJson(w.AuthLogin))
	route.POST("/api/auth/refresh", web.WithJson(w.AuthRefresh))
}
