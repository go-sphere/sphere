package render

import (
	"buf.build/go/protovalidate"
	"errors"
	"github.com/TBXark/sphere/database/mapper"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/server/ginx"
	"strings"
)

func init() {
	ginx.SetDefaultErrorParser(func(err error) (int32, int32, string) {
		var ve *protovalidate.ValidationError
		if errors.As(err, &ve) {
			return ValidationError(ve)
		}
		var ne *ent.NotFoundError
		if errors.As(err, &ne) {
			return EntNotFoundError(ne)
		}
		var ce *ent.ConstraintError
		if errors.As(err, &ce) {
			return EntConstraintError(ce)
		}
		return ginx.ParseError(err)
	})
}

func ValidationError(err *protovalidate.ValidationError) (int32, int32, string) {
	return 0, 400, strings.Join(mapper.Map(err.Violations, func(s *protovalidate.Violation) string {
		return s.Proto.GetMessage()
	}), ",")
}

func EntNotFoundError(err *ent.NotFoundError) (int32, int32, string) {
	return 0, 404, err.Error()
}

func EntConstraintError(err *ent.ConstraintError) (int32, int32, string) {
	return 0, 400, err.Unwrap().Error()
}
