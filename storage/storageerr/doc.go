// Package storageerr holds the shared sentinel errors returned by every
// storage driver (local, s3, qiniu, kvcache, fileserver).
//
// The sentinels are plain errors with no transport semantics: they carry no
// HTTP status. The HTTP mapping (ErrNotFound → 404, ErrDestExists and
// ErrFileNameInvalid → 400) lives in server/httpz.ParseError, the default
// error parser.
//
// Drivers may return these values directly or wrapped, so compare with
// errors.Is rather than ==:
//
//	import (
//		"errors"
//
//		"github.com/go-sphere/sphere/storage/storageerr"
//	)
//
//	res, err := store.DownloadFile(ctx, key)
//	switch {
//	case errors.Is(err, storageerr.ErrNotFound):
//		// the object does not exist
//	case errors.Is(err, storageerr.ErrFileNameInvalid):
//		// the key was rejected by storage.NormalizeKey
//	case err != nil:
//		return err
//	}
//	defer res.Reader.Close()
package storageerr
