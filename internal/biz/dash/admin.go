package dash

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/dao/ent/admin"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/auth/jwtauth"
	"github.com/tbxark/go-base-api/pkg/web/webmodels"
)

const WebPermissionAdmin = "admin"

type AdminListResponse struct {
	Admins []*render.Admin `json:"admins"`
}

// AdminList
// @Summary 管理员列表
// @Tags dashboard
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[AdminListResponse]
// @Router /api/admin/list [get]
func (w *Web) AdminList(ctx *gin.Context) (*AdminListResponse, error) {
	all, err := w.db.Admin.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return &AdminListResponse{
		Admins: lo.Map(all, func(a *ent.Admin, i int) *render.Admin {
			return render.AdminWithRoles(a)
		}),
	}, nil
}

type AdminEditRequest struct {
	Avatar   string   `json:"avatar" validate:"url"`
	Username string   `json:"username" validate:"required,min=3,max=50"`
	Nickname string   `json:"nickname" validate:"required,min=2,max=50"`
	Password string   `json:"password" validate:"omitempty,min=8"`
	Roles    []string `json:"roles" validate:"required,min=1"`
}

type AdminInfoResponse struct {
	Admin *render.Admin `json:"admin"`
}

// AdminCreate
// @Summary 创建管理员
// @Tags dashboard
// @Accept json
// @Produce json
// @Param admin body AdminEditRequest true "管理员信息"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[AdminInfoResponse]
// @Router /api/admin/create [post]
func (w *Web) AdminCreate(ctx *gin.Context) (*AdminInfoResponse, error) {
	var req AdminEditRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	if len(req.Password) > 8 {
		req.Password = encrypt.CryptPassword(req.Password)
	} else {
		return nil, webmodels.NewHTTPError(400, "password is too short")
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
	return &AdminInfoResponse{
		Admin: render.AdminWithRoles(u),
	}, nil
}

// AdminUpdate
// @Summary 更新管理员
// @Tags dashboard
// @Accept json
// @Produce json
// @Param admin body AdminEditRequest true "管理员信息"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[AdminInfoResponse]
// @Router /api/admin/update/{id} [post]
func (w *Web) AdminUpdate(ctx *gin.Context) (*AdminInfoResponse, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return nil, err
	}
	var req AdminEditRequest
	if e := ctx.ShouldBindJSON(&req); e != nil {
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
			return nil, webmodels.NewHTTPError(400, "password is too short")
		}
	}
	u, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return &AdminInfoResponse{
		Admin: render.AdminWithRoles(u),
	}, nil
}

func (w *Web) getAdminByID(ctx *gin.Context, idParam string) (*ent.Admin, error) {
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return nil, err
	}

	adm, err := w.db.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return adm, nil
}

// AdminDetail
// @Summary 管理员详情
// @Tags dashboard
// @Produce json
// @Param id path int true "管理员ID"
// @Security ApiKeyAuth
// @Success 200 {object} web.DataResponse[AdminInfoResponse]
// @Router /api/admin/detail/{id} [get]
func (w *Web) AdminDetail(ctx *gin.Context) (*AdminInfoResponse, error) {
	adm, err := w.getAdminByID(ctx, ctx.Param("id"))
	if err != nil {
		return nil, err
	}
	return &AdminInfoResponse{
		Admin: render.AdminWithRoles(adm),
	}, nil
}

// AdminDelete
// @Summary 删除管理员
// @Tags dashboard
// @Produce json
// @Param id path int true "管理员ID"
// @Security ApiKeyAuth
// @Success 200 {object} MessageResponse
// @Router /api/admin/delete/{id} [delete]
func (w *Web) AdminDelete(ctx *gin.Context) (*webmodels.MessageResponse, error) {
	adm, err := w.getAdminByID(ctx, ctx.Param("id"))
	if err != nil {
		return nil, err
	}

	value, exists := ctx.Get("username")
	if !exists {
		return nil, webmodels.NewHTTPError(400, "username not found")
	}
	if adm.Username == value.(string) {
		return nil, webmodels.NewHTTPError(400, "can not delete self")
	}
	err = w.db.Admin.DeleteOneID(adm.ID).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return webmodels.NewSuccessResponse(), nil
}

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
// @Success 200 {object} web.DataResponse[AdminLoginResponse]
// @Router /api/admin/login [post]
func (w *Web) AdminLogin(ctx *gin.Context) (*AdminLoginResponse, error) {
	var req AdminLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	u, err := w.db.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if !encrypt.IsPasswordMatch(req.Password, u.Password) {
		return nil, webmodels.NewHTTPError(400, "password not match")
	}
	return w.createLoginResponse(u)
}

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
	claims, err := w.token.Validate(body.RefreshToken)
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
	route := r.Group("/", w.auth.NewPermissionMiddleware(WebPermissionAdmin))
	route.GET("/api/admin/list", web.WithJson(w.AdminList))
	route.POST("/api/admin/create", web.WithJson(w.AdminCreate))
	route.POST("/api/admin/update/:id", web.WithJson(w.AdminUpdate))
	route.GET("/api/admin/detail/:id", web.WithJson(w.AdminDetail))
	route.DELETE("/api/admin/delete/:id", web.WithJson(w.AdminDelete))
}
