package storageerr

import "errors"

var (
	ErrorNotFound        = errors.New("key not found")
	ErrorDistExisted     = errors.New("destination key existed")
	ErrorFileNameInvalid = errors.New("file name invalid")
)
