package dash

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/dao/ent/admin"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/model"
	"strconv"
)

const WebPermissionAdmin = "admin"

// AdminList
// @Summary 管理员列表
// @Tags dashboard
// @Produce json
// @Param Authorization header string true "Bearer token"
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
	ID       int      `json:"id"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

// AdminCreate
// @Summary 创建管理员
// @Tags dashboard
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param admin body AdminEditRequest true "管理员信息"
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
		SetUsername(req.Username).
		SetPassword(req.Password).
		SetRoles(req.Roles).Save(ctx)
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
// @Param Authorization header string true "Bearer token"
// @Param admin body AdminEditRequest true "管理员信息"
// @Success 200 {object} render.Admin
// @Router /api/admin/update [post]
func (w *Web) AdminUpdate(ctx *gin.Context) (gin.H, error) {
	var req AdminEditRequest
	if err := ctx.BindJSON(&req); err != nil {
		return nil, err
	}
	update := w.db.Admin.UpdateOneID(req.ID).
		SetUsername(req.Username).
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
// @Param Authorization header string true "Bearer token"
// @Param id path int true "管理员ID"
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
// @Param Authorization header string true "Bearer token"
// @Param id path int true "管理员ID"
// @Success 200 {object} model.MessageResponse
// @Router /api/admin/delete/{id} [post]
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
	Token      string   `json:"token"`
	Permission []string `json:"permission"`
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
	tokenString, err := w.auth.CreateJwtToken(u.ID, u.Username, u.Roles...)
	return &AdminLoginResponse{
		Token:      tokenString,
		Permission: u.Roles,
	}, nil
}

func (w *Web) bindAdminAuthRoute(r gin.IRouter) {
	r.POST("/api/admin/login", web.WithJson(w.AdminLogin))
}

func (w *Web) bindAdminRoute(r gin.IRouter) {
	r.GET("/api/admin/list", web.WithJson(w.AdminList))
	r.POST("/api/admin/create", web.WithJson(w.AdminCreate))
	r.POST("/api/admin/update", web.WithJson(w.AdminUpdate))
	r.GET("/api/admin/detail/:id", web.WithJson(w.AdminDetail))
	r.POST("/api/admin/delete/:id", web.WithJson(w.AdminDelete))
}
