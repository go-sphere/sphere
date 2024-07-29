package dash

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/dao/ent/admin"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/auth/tokens"
	"github.com/tbxark/go-base-api/pkg/web/model"
	"strconv"
	"time"
)

const WebPermissionAdmin = "admin"

// AdminList
// @Summary 管理员列表
// @Tags dashboard
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} []render.Admin
// @Router /api/admin/list [get]
func (w *Web) AdminList(ctx *gin.Context) (gin.H, error) {
	all, err := w.db.Admin.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"admins": lo.Map(all, func(a *ent.Admin, i int) *render.Admin {
			return render.AdminWithRoles(a)
		}),
	}, nil
}

type AdminEditRequest struct {
	Avatar   string   `json:"avatar"`
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

// AdminCreate
// @Summary 创建管理员
// @Tags dashboard
// @Accept json
// @Produce json
// @Param admin body AdminEditRequest true "管理员信息"
// @Security ApiKeyAuth
// @Success 200 {object} render.Admin
// @Router /api/admin/create [post]
func (w *Web) AdminCreate(ctx *gin.Context) (gin.H, error) {
	var req AdminEditRequest
	if err := ctx.BindJSON(&req); err != nil {
		return nil, err
	}
	if len(req.Password) > 8 {
		req.Password = encrypt.CryptPassword(req.Password)
	} else {
		return nil, model.NewHTTPError(400, "password is too short")
	}
	u, err := w.db.Admin.Create().
		SetAvatar(w.cdn.KeyFromURL(req.Avatar)).
		SetUsername(req.Username).
		SetNickname(req.Nickname).
		SetPassword(req.Password).
		SetRoles(req.Roles).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"admin": render.AdminWithRoles(u),
	}, nil
}

// AdminUpdate
// @Summary 更新管理员
// @Tags dashboard
// @Accept json
// @Produce json
// @Param admin body AdminEditRequest true "管理员信息"
// @Security ApiKeyAuth
// @Success 200 {object} render.Admin
// @Router /api/admin/update/{id} [post]
func (w *Web) AdminUpdate(ctx *gin.Context) (gin.H, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return nil, err
	}
	var req AdminEditRequest
	if e := ctx.BindJSON(&req); e != nil {
		return nil, e
	}
	update := w.db.Admin.UpdateOneID(id).
		SetAvatar(w.cdn.KeyFromURL(req.Avatar)).
		SetUsername(req.Username).
		SetNickname(req.Nickname).
		SetRoles(req.Roles)
	if req.Password != "" {
		update = update.SetPassword(encrypt.CryptPassword(req.Password))
		if len(req.Password) > 8 {
			req.Password = encrypt.CryptPassword(req.Password)
		} else {
			return nil, model.NewHTTPError(400, "password is too short")
		}
	}
	u, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"admin": render.AdminWithRoles(u),
	}, nil
}

// AdminDetail
// @Summary 管理员详情
// @Tags dashboard
// @Produce json
// @Param id path int true "管理员ID"
// @Security ApiKeyAuth
// @Success 200 {object} render.Admin
// @Router /api/admin/detail/{id} [get]
func (w *Web) AdminDetail(ctx *gin.Context) (gin.H, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return nil, err
	}
	user, err := w.db.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"admin": render.AdminWithRoles(user),
	}, nil
}

// AdminDelete
// @Summary 删除管理员
// @Tags dashboard
// @Produce json
// @Param id path int true "管理员ID"
// @Security ApiKeyAuth
// @Success 200 {object} model.MessageResponse
// @Router /api/admin/delete/{id} [delete]
func (w *Web) AdminDelete(ctx *gin.Context) (*model.MessageResponse, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return nil, err
	}
	user, err := w.db.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	value, exists := ctx.Get("username")
	if !exists {
		return nil, model.NewHTTPError(400, "username not found")
	}
	if user.Username == value.(string) {
		return nil, model.NewHTTPError(400, "can not delete self")
	}
	err = w.db.Admin.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return model.NewSuccessResponse(), nil
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
	token, err := w.token.GenerateSignedToken(id, u.Username, u.Roles...)
	if err != nil {
		return nil, err
	}
	refresh, err := w.token.GenerateRefreshToken(id)
	if err != nil {
		return nil, err
	}
	return &AdminLoginResponse{
		Avatar:       w.cdn.RenderImageURL(u.Avatar, 512),
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
// @Success 200 {object} AdminLoginResponse
// @Router /api/admin/login [post]
func (w *Web) AdminLogin(ctx *gin.Context) (*AdminLoginResponse, error) {
	var req AdminLoginRequest
	if err := ctx.BindJSON(&req); err != nil {
		return nil, err
	}
	u, err := w.db.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if !encrypt.IsPasswordMatch(req.Password, u.Password) {
		return nil, model.NewHTTPError(400, "password not match")
	}
	return w.createLoginResponse(u)
}

type AdminRefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
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
// @Success 200 {object} AdminLoginResponse
// @Router /api/admin/refresh-token [post]
func (w *Web) AdminRefreshToken(ctx *gin.Context) (*AdminLoginResponse, error) {
	var body AdminRefreshTokenRequest
	if err := ctx.BindJSON(&body); err != nil {
		return nil, err
	}
	claims, err := w.token.Validate(body.RefreshToken)
	if err != nil {
		return nil, err
	}
	uid, ok := claims[tokens.SignedDetailsUidKey].(string)
	if !ok {
		return nil, model.NewHTTPError(400, "uid not found")
	}
	id, err := strconv.Atoi(uid)
	if err != nil {
		return nil, err
	}
	u, err := w.db.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return w.createLoginResponse(u)
}

func (w *Web) bindAdminAuthRoute(r gin.IRouter) {
	r.POST("/api/admin/login", web.WithJson(w.AdminLogin))
	r.POST("/api/admin/refresh-token", web.WithJson(w.AdminRefreshToken))
}

func (w *Web) bindAdminRoute(r gin.IRouter) {
	r.GET("/api/admin/list", web.WithJson(w.AdminList))
	r.POST("/api/admin/create", web.WithJson(w.AdminCreate))
	r.POST("/api/admin/update/:id", web.WithJson(w.AdminUpdate))
	r.GET("/api/admin/detail/:id", web.WithJson(w.AdminDetail))
	r.DELETE("/api/admin/delete/:id", web.WithJson(w.AdminDelete))
}
