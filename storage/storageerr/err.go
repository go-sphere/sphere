package storageerr

import "errors"

var (
	ErrorNotFound        = errors.New("key not found")
	ErrorDistExisted     = errors.New("destination key existed")
	ErrorSourceNotFound  = errors.New("source file not found")
	ErrorFileNameInvalid = errors.New("file name invalid")
)
