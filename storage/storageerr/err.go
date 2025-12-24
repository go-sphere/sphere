package storageerr

import (
	"errors"

	"github.com/go-sphere/httpx"
)

// Common storage operation errors with appropriate HTTP status codes.
var (
	// ErrorNotFound indicates that the requested storage key does not exist.
	ErrorNotFound = httpx.NotFoundError(errors.New("key not found"))

	// ErrorDistExisted indicates that the destination key already exists when overwrite is disabled.
	ErrorDistExisted = httpx.BadRequestError(errors.New("destination key existed"))

	// ErrorFileNameInvalid indicates that the provided file name or path is invalid or unsafe.
	ErrorFileNameInvalid = httpx.BadRequestError(errors.New("file name invalid"))
)
