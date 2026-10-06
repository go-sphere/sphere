// Package cors provides configurable CORS middleware for httpx.
//
// Build the middleware once with [NewCORS] and register it before routing
// middleware such as auth, so preflight requests are answered without
// credentials.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/server/middleware/cors"
//	)
//
//	corsMiddleware, err := cors.NewCORS(
//		cors.WithAllowOrigins("https://app.example.com", "https://*.example.com"),
//		cors.WithAllowCredentials(true),
//		cors.WithMaxAge(12*time.Hour),
//	)
//	if err != nil {
//		return err // cors.ErrWildcardWithCredentials
//	}
//	engine := stdx.New()
//	engine.Use(corsMiddleware)
//
// # Behavior
//
//   - Every OPTIONS request is answered with 204 and the CORS headers; it
//     never reaches the route handler.
//   - With no WithAllowOrigins option no origin matches, so browsers block
//     cross-origin reads. "*" allows any origin; patterns such as
//     "https://*.example.com" or a bare host ("app.example.com") echo the
//     matching request origin.
//   - A bare "*" combined with credentials is rejected at construction with
//     [ErrWildcardWithCredentials].
//   - Default methods are GET, POST, PUT, DELETE, PATCH, and OPTIONS. Without
//     WithAllowHeaders the requested headers are echoed, or a default list when
//     the request names none.
//   - Vary: Origin is set unless the origin list contains "*".
package cors
