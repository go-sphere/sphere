package dash

import (
	"context"
	"time"

	"github.com/TBXark/sphere/database/mapper"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/adminsession"
)

var _ dashv1.AdminSessionServiceHTTPServer = (*Service)(nil)

func (s *Service) DeleteAdminSession(ctx context.Context, request *dashv1.DeleteAdminSessionRequest) (*dashv1.DeleteAdminSessionResponse, error) {
	err := s.db.AdminSession.UpdateOneID(request.Id).SetIsRevoked(true).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.DeleteAdminSessionResponse{}, nil
}

func (s *Service) ListAdminSessions(ctx context.Context, request *dashv1.ListAdminSessionsRequest) (*dashv1.ListAdminSessionsResponse, error) {
	uid, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	query := s.db.AdminSession.Query().Where(adminsession.UIDEQ(uid))
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	page, size := mapper.Page(count, int(request.PageSize), mapper.DefaultPageSize)
	all, err := query.Clone().Limit(size).Offset(size * int(request.Page)).All(ctx)
	if err != nil {
		return nil, err
	}
	revoked := make([]int64, 0, len(all))
	for _, session := range all {
		if !session.IsRevoked && session.Expires < time.Now().Unix() {
			session.IsRevoked = true
			revoked = append(revoked, session.ID)
		}
	}
	if len(revoked) > 0 {
		_ = s.db.AdminSession.Update().Where(adminsession.IDIn(revoked...)).SetIsRevoked(true).Exec(ctx)
	}
	return &dashv1.ListAdminSessionsResponse{
		AdminSessions: mapper.Map(all, s.render.AdminSession),
		TotalSize:     int64(count),
		TotalPage:     int64(page),
	}, nil
}
