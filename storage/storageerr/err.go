package storageerr

import "errors"

// Common storage operation errors.
var (
	// ErrNotFound indicates that the requested storage key does not exist.
	// DeleteFile never returns it (deletion of a missing key succeeds), and
	// IsFileExists reports a missing key as false rather than as this error.
	ErrNotFound = errors.New("key not found")

	// ErrDestExists indicates that the destination key already exists when overwrite is disabled.
	ErrDestExists = errors.New("destination key existed")

	// ErrFileNameInvalid indicates that the provided file name or path is invalid or unsafe.
	ErrFileNameInvalid = errors.New("file name invalid")
)
