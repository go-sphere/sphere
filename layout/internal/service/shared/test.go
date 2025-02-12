package shared

import (
	"context"
	sharedv2 "github.com/TBXark/sphere/layout/api/shared/v1"
)

var _ sharedv2.TestServiceHTTPServer = (*Service)(nil)

func (s *Service) RunTest(ctx context.Context, req *sharedv2.RunTestRequest) (*sharedv2.RunTestResponse, error) {
	return &sharedv2.RunTestResponse{
		FieldTest1:	req.FieldTest1,
		FieldTest2:	req.FieldTest2,
		PathTest1:	req.PathTest1,
		PathTest2:	req.PathTest2,
		QueryTest1:	req.QueryTest1,
		QueryTest2:	req.QueryTest2,
	}, nil
}
