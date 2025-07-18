package shared

import (
	"context"

	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
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
		EnumTest1:  req.EnumTest1,
	}, nil
}

func (s *Service) BodyPathTest(ctx context.Context, request *sharedv1.BodyPathTestRequest) (*sharedv1.BodyPathTestResponse, error) {
	return &sharedv1.BodyPathTestResponse{
		Response: []*sharedv1.BodyPathTestResponse_Response{
			{
				FieldTest1: request.Request.FieldTest1,
				FieldTest2: request.Request.FieldTest2,
			},
		},
	}, nil
}
