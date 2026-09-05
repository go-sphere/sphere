package httpz

import (
	"path"
	"strings"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

func lastChar(s string) byte {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}
	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}

// EndpointsToMatches indexes generated routes as method → fullPath → operation.
// Each route is [3]string{operation, method, path}; path is joined onto base,
// preserving a trailing slash.
//
// Named-wildcard routes ("/files/*name") are indexed twice: once verbatim and
// once in anonymous form ("/files/*"). Adapters without named wildcards
// (echox, fiberx) rewrite the pattern at registration time and report the
// anonymous form from FullPath, so both dialects must resolve to the
// operation.
func EndpointsToMatches(base string, endpoints ...[][3]string) map[string]map[string]string {
	matches := make(map[string]map[string]string)
	for _, list := range endpoints {
		for _, route := range list {
			key := route[1]
			inner, ok := matches[key]
			if !ok || inner == nil {
				inner = make(map[string]string)
				matches[key] = inner
			}
			full := joinPaths(base, route[2])
			inner[full] = route[0]
			if anon := anonymousWildcardPath(full); anon != full {
				inner[anon] = route[0]
			}
		}
	}
	return matches
}

// anonymousWildcardPath rewrites a trailing named wildcard to its anonymous
// form ("/files/*name" -> "/files/*"), mirroring the registration-time
// rewrite httpx.FixWildcardPathIfNeed applies on adapters without named
// wildcards. Paths without a trailing single wildcard are returned unchanged.
func anonymousWildcardPath(p string) string {
	i := strings.LastIndexByte(p, '*')
	if i <= 0 || p[i-1] != '/' || strings.ContainsRune(p[i+1:], '/') {
		return p
	}
	return p[:i+1]
}

// MatchOperation returns a predicate that is true when the request's method
// and full path map to one of the named operations. Use it with
// middleware/selector to apply auth only to generated private routes.
//
// The predicate fails closed: when the route pattern cannot be determined
// (FullPath returns ""), it reports true so selector-gated middleware such as
// rate limiting or auth still runs, rather than being silently skipped.
// Because of that, do NOT compose it with selector.NewLogicalNotMatcher to
// express "everything except X" — an indeterminate route would then skip the
// middleware. Select X directly and apply the middleware to the outer group
// instead.
func MatchOperation(base string, endpoints [][3]string, operations ...string) func(ctx httpx.Context) bool {
	matches := EndpointsToMatches(base, endpoints)
	opts := make(map[string]struct{}, len(operations))
	for _, opt := range operations {
		opts[opt] = struct{}{}
	}
	return func(ctx httpx.Context) bool {
		method, ok := matches[ctx.Method()]
		if !ok {
			// No generated endpoint uses this method at all, so the request
			// determinately does not belong to the operation set.
			return false
		}
		fullPath := ctx.FullPath()
		if fullPath == "" {
			// Fail closed: without a route pattern the operation cannot be
			// identified. Treat it as a hit so rate limiting/auth still runs;
			// silently skipping them is the dangerous direction. See audit B7.
			log.Warn("httpz: MatchOperation cannot determine route pattern; failing closed",
				log.String("method", ctx.Method()), log.String("path", ctx.Path()))
			return true
		}
		opt, ok := method[fullPath]
		if !ok {
			return false
		}
		_, ok = opts[opt]
		return ok
	}
}
