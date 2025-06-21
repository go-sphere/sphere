package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TBXark/sphere/core/errors/statuserr"
	"github.com/TBXark/sphere/core/safe"
	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/auth"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/userplatform"
	"github.com/TBXark/sphere/social/wechat"
	"github.com/TBXark/sphere/storage"
)

var _ apiv1.UserServiceHTTPServer = (*Service)(nil)

var wechatAvatarDomains = map[string]struct{}{
	"thirdwx.qlogo.cn": {},
}

const RemoteImageMaxSize = 1024 * 1024 * 2

var (
	ErrImageSizeExceed     = fmt.Errorf("image size exceed")
	ErrImageHostNotAllowed = fmt.Errorf("image host not allowed")
)

func (s *Service) GetMineInfo(ctx context.Context, req *apiv1.GetMineInfoRequest) (*apiv1.GetMineInfoResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.db.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetMineInfoResponse{
		User: s.render.Me(me),
	}, nil
}

func (s *Service) GetMinePlatform(ctx context.Context, request *apiv1.GetMinePlatformRequest) (*apiv1.GetMinePlatformResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.db.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	plat, err := s.db.UserPlatform.Query().Where(userplatform.UserIDEQ(id)).All(ctx)
	if err != nil {
		return nil, err
	}
	res := apiv1.GetMinePlatformResponse{
		Username: me.Username,
	}
	for _, p := range plat {
		switch p.Platform {
		case auth.PlatformWechatMini:
			res.WechatMini = p.PlatformID
		case auth.PlatformPhone:
			res.Phone = p.PlatformID
		}
	}
	return &res, nil
}

func (s *Service) UpdateMineInfo(ctx context.Context, req *apiv1.UpdateMineInfoRequest) (*apiv1.UpdateMineInfoResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	req.Avatar, err = s.uploadRemoteImage(ctx, req.Avatar)
	if err != nil {
		return nil, err
	}
	req.Avatar = s.storage.ExtractKeyFromURL(req.Avatar)
	up, err := s.db.User.UpdateOneID(id).
		SetUsername(req.Username).
		SetAvatar(req.Avatar).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &apiv1.UpdateMineInfoResponse{
		User: s.render.Me(up),
	}, nil
}

func (s *Service) BindPhoneWxMini(ctx context.Context, req *apiv1.BindPhoneWxMiniRequest) (*apiv1.BindPhoneWxMiniResponse, error) {
	userId, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	number, err := s.wechat.GetUserPhoneNumber(ctx, req.Code, wechat.WithRetryable(true))
	if err != nil {
		return nil, err
	}
	if number.PhoneInfo.CountryCode != "86" {
		return nil, statuserr.BadRequestError(errors.New("only support China phone number"), "仅支持中国大陆手机号")
	}
	err = s.db.UserPlatform.Create().
		SetUserID(userId).
		SetPlatform(auth.PlatformPhone).
		SetPlatformID(number.PhoneInfo.PhoneNumber).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &apiv1.BindPhoneWxMiniResponse{}, nil
}

func (s *Service) uploadRemoteImage(ctx context.Context, uri string) (string, error) {
	key, err := s.storage.ExtractKeyFromURLWithMode(uri, true)
	if key != "" && err == nil {
		return key, nil
	}
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	isValidHost := false
	for domain := range wechatAvatarDomains {
		if strings.HasSuffix(u.Host, domain) {
			isValidHost = true
			break
		}
	}
	if !isValidHost {
		return "", ErrImageHostNotAllowed
	}
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return "", err
	}
	key = storage.DefaultKeyBuilder(strconv.Itoa(int(id)))(uri, "user")
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.ContentLength > RemoteImageMaxSize {
		return "", ErrImageSizeExceed
	}
	defer safe.IfErrorPresent("close response body", resp.Body.Close)
	ret, err := s.storage.UploadFile(ctx, resp.Body, key)
	if err != nil {
		return "", err
	}
	return ret, nil
}
