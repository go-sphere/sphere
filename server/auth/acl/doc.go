// Package acl is a static in-memory allow-list mapping subject → resource.
// It is fail-closed: an unknown subject or resource is denied, and there is no
// deny or revoke API.
//
// [ACL] satisfies server/middleware/auth.AccessControl, whose IsAllowed takes
// a role as the subject, so it is the usual backing store for
// auth.NewPermissionMiddleware.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/server/auth/acl"
//
//	permissions := acl.NewACL()
//	permissions.Allow("admin", "orders:write")
//	permissions.Allow("admin", "orders:read")
//	permissions.Allow("viewer", "orders:read")
//
//	permissions.IsAllowed("viewer", "orders:write") // false
//
// Populate the ACL during startup, before serving requests. Calling Allow
// concurrently with Allow or IsAllowed is a data race; after setup, concurrent
// IsAllowed calls are safe.
package acl
