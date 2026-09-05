package httpz

import (
	"fmt"
	"net/http"

	"github.com/go-sphere/httpx"
)

// stdMountMethods is the default method set for MountStdAll: the portable
// subset of the httpx Registrar contract, without CONNECT/TRACE whose routing
// semantics are framework-dependent.
var stdMountMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
	http.MethodPatch, http.MethodDelete, http.MethodOptions,
}

// MountStdAll mounts a net/http handler on r at path for the given methods
// (defaulting to GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS). It fails loud with
// an error when r does not support httpx.StdHandlerMounter instead of
// silently dropping routes; all official httpx adapters support it.
func MountStdAll(r httpx.Registrar, path string, h http.Handler, methods ...string) error {
	if len(methods) == 0 {
		methods = stdMountMethods
	}
	for _, m := range methods {
		if !httpx.MountStd(r, m, path, h) {
			return fmt.Errorf("httpz: registrar %T does not support mounting net/http handlers", r)
		}
	}
	return nil
}
