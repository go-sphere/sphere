package dashv1

import (
	"github.com/bufbuild/protovalidate-go"
	"testing"
)

func TestAuthLoginRequest_GetPassword(t *testing.T) {
	msg := &AuthLoginRequest{
		Password: "123456",
	}
	if err := protovalidate.Validate(msg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
