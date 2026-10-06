// Package authorizer is the identity contract shared by token implementations
// and HTTP auth middleware. Token format is out of scope:
// server/auth/jwtauth implements [Parser] and [Generator] for JWT, and
// server/middleware/auth turns a [Parser] into request middleware.
//
// The identity of a request is a [Data] value stored on its
// [context.Context]. Auth middleware stores it with [WithAuthData]; business
// code reads it through [ContextUtils], parameterized with the same [UID]
// type, instead of calling context.Value directly.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/server/auth/authorizer"
//
//	var auth authorizer.ContextUtils[int64]
//
//	func (s *Service) ListMyOrders(ctx context.Context) ([]Order, error) {
//		uid, err := auth.GetCurrentID(ctx) // NeedLoginError when unauthenticated
//		if err != nil {
//			return nil, err
//		}
//		return s.orders.ListByOwner(ctx, uid)
//	}
//
// [ContextUtils.CheckAuthID] additionally returns [PermissionError] when the
// current user is not the owner of a resource.
//
// # Contract
//
//   - [UID] is an integer or string type. Store uuid.UUID and similar IDs in
//     their String() form.
//   - [Claims.GetUID] errors reject the request and a zero UID never
//     authenticates; GetSubject and GetRoles errors leave zero values.
//   - Data stored under one UID type is invisible to readers using another.
//
// Sentinel errors carry an HTTP status and a Chinese user-facing message:
// [TokenNotFoundError], [NeedLoginError], and [MissingUIDError] are 401;
// [PermissionError] is 403.
package authorizer
