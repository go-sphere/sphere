package api

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/internal/pkg/consts"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/internal/pkg/database/ent"
	"github.com/tbxark/go-base-api/internal/pkg/database/ent/userplatform"
	"github.com/tbxark/go-base-api/pkg/web"
	"strconv"
	"time"
)

type WxMiniAuthRequest struct {
	Code string `json:"code" binding:"required"`
}

type AuthResponse struct {
	IsNew bool      `json:"isNew"`
	Token string    `json:"token"`
	User  *ent.User `json:"user"`
}

// AuthWxMini
//
//	@Summary	微信小程序登录
//	@Param		request	body		WxMiniAuthRequest	true	"登录信息"
//	@Success	200		{object}	web.DataResponse[AuthResponse]
//	@Router		/api/auth/wxmini [post]
func (w *Web) AuthWxMini(ctx *gin.Context) (*AuthResponse, error) {
	var req WxMiniAuthRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	wxUser, err := w.Wechat.Auth(req.Code)
	if err != nil {
		return nil, err
	}

	res, err := dao.WithTx[AuthResponse](ctx, w.DB.Client, func(ctx context.Context, client *ent.Client) (*AuthResponse, error) {
		userPlat, e := client.UserPlatform.Query().
			Where(userplatform.PlatformEQ(consts.WechatMiniPlatform), userplatform.PlatformIDEQ(wxUser.OpenID)).
			Only(ctx)
		// 用户存在
		if e == nil && userPlat != nil {
			u, ue := client.User.Get(ctx, userPlat.UserID)
			if ue != nil {
				return nil, ue
			}
			return &AuthResponse{
				User:  u,
				IsNew: false,
			}, nil
		}
		// 其他错误
		if !ent.IsNotFound(e) {
			return nil, e
		}
		// 用户不存在
		newUser, e := client.User.Create().
			SetUsername(fmt.Sprintf("微信用户%d", time.Now().Unix()/1000)).
			SetAvatar(consts.DefaultUserAvatar).
			Save(ctx)
		if e != nil {
			return nil, e
		}
		_, e = client.UserPlatform.Create().SetUserID(newUser.ID).
			SetPlatform(consts.WechatMiniPlatform).
			SetPlatformID(wxUser.OpenID).
			SetSecondID(wxUser.UnionID).
			Save(ctx)
		if e != nil {
			return nil, e
		}
		return &AuthResponse{
			User:  newUser,
			IsNew: true,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	token, err := w.JwtAuth.GenerateSignedToken(strconv.Itoa(res.User.ID), consts.WechatMiniPlatform+":"+wxUser.OpenID)
	if err != nil {
		return nil, err
	}
	res.Token = token.Token
	res.User = w.Render.Me(res.User)
	return res, nil
}

func (w *Web) bindAuthRoute(r gin.IRouter) {
	route := r.Group("/")
	route.POST("/api/auth/wxmini", web.WithJson(w.AuthWxMini))
}
