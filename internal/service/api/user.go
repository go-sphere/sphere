package api

import (
	"context"
	"fmt"
	apiv1 "github.com/tbxark/sphere/api/api/v1"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/user"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/web/statuserr"
	"strconv"
	"strings"
)

var _ apiv1.UserServiceHTTPServer = (*Service)(nil)

const RemoteImageMaxSize = 1024 * 1024 * 2

var ErrImageSizeExceed = fmt.Errorf("image size exceed")

func (s *Service) BindPhoneWxMini(ctx context.Context, req *apiv1.BindPhoneWxMiniRequest) (*apiv1.BindPhoneWxMiniResponse, error) {
	userId, err := s.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	number, err := s.Wechat.GetUserPhoneNumber(req.Code, true)
	if err != nil {
		return nil, err
	}
	if number.PhoneInfo.CountryCode != "86" {
		return nil, statuserr.NewHTTPError(400, "只支持中国大陆手机号")
	}
	err = dao.WithTxEx(ctx, s.DB.Client, func(ctx context.Context, client *ent.Client) error {
		exist, e := client.User.Query().Where(user.PhoneEQ(number.PhoneInfo.PhoneNumber)).Only(ctx)
		if e != nil {
			if ent.IsNotFound(e) {
				_, ue := client.User.UpdateOneID(userId).SetPhone(number.PhoneInfo.PhoneNumber).Save(ctx)
				return ue
			}
			return e
		}
		if exist.ID != userId {
			return statuserr.NewHTTPError(400, "手机号已被绑定")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &apiv1.BindPhoneWxMiniResponse{}, nil
}

func (s *Service) Me(ctx context.Context, req *apiv1.MeRequest) (*apiv1.MeResponse, error) {
	id, err := s.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.DB.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &apiv1.MeResponse{
		User: s.Render.Me(me),
	}, nil
}

func (s *Service) Update(ctx context.Context, req *apiv1.UpdateRequest) (*apiv1.UpdateResponse, error) {
	id, err := s.Auth.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	req.Avatar, err = s.uploadRemoteImage(ctx, req.Avatar)
	if err != nil {
		return nil, err
	}
	req.Avatar = s.Storage.ExtractKeyFromURL(req.Avatar)
	up, err := s.DB.User.UpdateOneID(id).
		SetUsername(req.Username).
		SetAvatar(req.Avatar).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &apiv1.UpdateResponse{
		User: s.Render.Me(up),
	}, nil
}

func (s *Service) uploadRemoteImage(ctx context.Context, url string) (string, error) {
	key := s.Storage.ExtractKeyFromURL(url)
	if key == "" {
		return key, nil
	}
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		return key, nil
	}
	id, err := s.Auth.GetCurrentID(ctx)
	if err != nil {
		return "", err
	}
	key = storage.DefaultKeyBuilder(strconv.Itoa(int(id)))(url, "user")
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	if resp.ContentLength > RemoteImageMaxSize {
		return "", ErrImageSizeExceed
	}
	defer resp.Body.Close()
	ret, err := s.Storage.UploadFile(ctx, resp.Body, resp.ContentLength, key)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}
