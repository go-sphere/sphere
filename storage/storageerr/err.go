package storageerr

import (
	"errors"

	"github.com/go-sphere/sphere/core/errors/statuserr"
)

var (
	ErrorNotFound        = statuserr.NotFoundError(errors.New("key not found"))
	ErrorDistExisted     = statuserr.BadRequestError(errors.New("destination key existed"))
	ErrorFileNameInvalid = statuserr.BadRequestError(errors.New("file name invalid"))
)
