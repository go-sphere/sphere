package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/user"
	"github.com/tbxark/sphere/internal/pkg/render"
	"github.com/tbxark/sphere/pkg/web/ginx"
)

type UserInfoMePlatform struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Join bool   `json:"join"`
}

type UserInfoMeResponse struct {
	Info    *render.User `json:"info"`
	Inviter *render.User `json:"inviter"`
}

// UserMe
//
// @Summary 获取当前用户信息
// @Success 200 {object} ginx.DataResponse[UserInfoMeResponse]
// @Router /api/user/me [get]
func (w *Web) UserMe(ctx *gin.Context) (*UserInfoMeResponse, error) {
	id, err := w.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := w.DB.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	res := UserInfoMeResponse{
		Info: w.Render.Me(me),
	}
	return &res, nil
}

type UpdateUserInfoRequest struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

type UpdateUserInfoResponse struct {
	Info *render.User `json:"info"`
}

// UserUpdate
//
// @Summary 更新用户信息
// @Param user body UpdateUserInfoRequest true "用户信息"
// @Success 200 {object} ginx.DataResponse[UpdateUserInfoResponse]
// @Router /api/user/update [post]
func (w *Web) UserUpdate(ctx *gin.Context) (*UpdateUserInfoResponse, error) {
	id, err := w.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	var info UpdateUserInfoRequest
	if e := ctx.ShouldBindJSON(&info); e != nil {
		return nil, e
	}
	info.Avatar, err = w.uploadRemoteImage(ctx, info.Avatar)
	if err != nil {
		return nil, err
	}
	info.Avatar = w.Storage.ExtractKeyFromURL(info.Avatar)
	up, err := w.DB.User.UpdateOneID(id).
		SetUsername(info.Username).
		SetAvatar(info.Avatar).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &UpdateUserInfoResponse{
		Info: w.Render.Me(up),
	}, nil
}

type WxMiniBindPhoneRequest struct {
	Code string `json:"code" binding:"required"`
}

// UserBindPhoneWxMini
//
// @Summary 绑定手机号
// @Param request body WxMiniBindPhoneRequest true "绑定信息"
// @Success 200 {object} ginx.MessageResponse
// @Router /api/user/bind/phone/wxmini [post]
func (w *Web) UserBindPhoneWxMini(ctx *gin.Context) (*ginx.SimpleMessage, error) {
	var req WxMiniBindPhoneRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	userId, err := w.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	number, err := w.Wechat.GetUserPhoneNumber(req.Code, true)
	if err != nil {
		return nil, err
	}
	if number.PhoneInfo.CountryCode != "86" {
		return nil, ginx.NewHTTPError(400, "只支持中国大陆手机号")
	}
	err = dao.WithTxEx(ctx, w.DB.Client, func(ctx context.Context, client *ent.Client) error {
		exist, e := client.User.Query().Where(user.PhoneEQ(number.PhoneInfo.PhoneNumber)).Only(ctx)
		if e != nil {
			if ent.IsNotFound(e) {
				_, ue := client.User.UpdateOneID(userId).SetPhone(number.PhoneInfo.PhoneNumber).Save(ctx)
				return ue
			}
			return e
		}
		if exist.ID != userId {
			return ginx.NewHTTPError(400, "手机号已被绑定")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ginx.NewSuccessResponse(), nil
}

func (w *Web) bindUserRoute(r gin.IRouter) {
	route := r.Group("/")
	route.GET("/api/user/me", ginx.WithJson(w.UserMe))
	route.POST("/api/user/update", ginx.WithJson(w.UserUpdate))
	route.POST("/api/user/bind/phone/wxmini", ginx.WithJson(w.UserBindPhoneWxMini))
}
