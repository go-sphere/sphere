package shared

import (
	"context"
	sharedv1 "github.com/TBXark/sphere/api/shared/v1"
)

var _ sharedv1.TestServiceHTTPServer = (*Service)(nil)

func (s *Service) RunTest(ctx context.Context, req *sharedv1.RunTestRequest) (*sharedv1.RunTestResponse, error) {
	return &sharedv1.RunTestResponse{
		FieldTest1: req.FieldTest1,
		FieldTest2: req.FieldTest2,
		PathTest1:  req.PathTest1,
		PathTest2:  req.PathTest2,
		QueryTest1: req.QueryTest1,
		QueryTest2: req.QueryTest2,
	}, nil
}
