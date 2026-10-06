// Package storageerr is the shared sentinel errors for storage drivers.
//
// The sentinels are plain errors with no transport semantics: they carry no
// HTTP status. The HTTP mapping (ErrNotFound → 404, ErrDestExists and
// ErrFileNameInvalid → 400) lives in server/httpz.ParseError, the default
// error parser.
//
// Use errors.Is to compare against these sentinels.
package storageerr

import "errors"

// Common storage operation errors.
var (
	// ErrNotFound indicates that the requested storage key does not exist.
	ErrNotFound = errors.New("key not found")

	// ErrDestExists indicates that the destination key already exists when overwrite is disabled.
	ErrDestExists = errors.New("destination key existed")

	// ErrFileNameInvalid indicates that the provided file name or path is invalid or unsafe.
	ErrFileNameInvalid = errors.New("file name invalid")
)
