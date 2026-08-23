// Package storageerr is the shared sentinel errors for storage drivers,
// wrapped with httpx status (NotFound / BadRequest).
//
// Use errors.Is. ErrorDistExisted is a deprecated alias of ErrDestExists.
package storageerr

import (
	"errors"

	"github.com/go-sphere/httpx"
)

// Common storage operation errors with appropriate HTTP status codes.
var (
	// ErrNotFound indicates that the requested storage key does not exist.
	ErrNotFound = httpx.NotFoundError(errors.New("key not found"))

	// ErrDestExists indicates that the destination key already exists when overwrite is disabled.
	ErrDestExists = httpx.BadRequestError(errors.New("destination key existed"))

	// ErrFileNameInvalid indicates that the provided file name or path is invalid or unsafe.
	ErrFileNameInvalid = httpx.BadRequestError(errors.New("file name invalid"))

	// Backwards-compatible aliases and alternate spellings.
	ErrorNotFound        = ErrNotFound
	ErrorDestExisted     = ErrDestExists
	ErrorDistExisted     = ErrDestExists // Deprecated: typo in original name, use ErrDestExists or ErrorDestExisted instead
	ErrorFileNameInvalid = ErrFileNameInvalid
)
