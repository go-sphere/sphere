package authorizer

import (
	"context"
	"slices"
)

type authKey struct{}

var authContextKey = authKey{}

// Data is the identity stored on the request context by auth middleware.
// UID is the authenticated user; Subject and Roles are optional and may be
// zero. Roles is stored as given, so callers must not mutate it after passing
// it to WithAuthData; ContextUtils.GetCurrentRoles returns a copy.
type Data[I UID] struct {
	UID     I        `json:"uid"`
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
}

// WithAuthData returns a child context holding data under the package's
// unexported key. A later WithAuthData on a derived context shadows it. It
// does not validate data; middleware that authenticates requests is
// responsible for rejecting a zero UID before storing it.
func WithAuthData[I UID](ctx context.Context, data Data[I]) context.Context {
	return context.WithValue(ctx, authContextKey, data)
}

// GetAuthData returns the identity stored by WithAuthData. A missing value
// or a Data stored under a different UID type yields (_, false).
func GetAuthData[I UID](ctx context.Context) (Data[I], bool) {
	data, ok := ctx.Value(authContextKey).(Data[I])
	return data, ok
}

// ContextUtils provides utility functions for working with authentication context.
// It is parameterized by the user ID type for type safety; I must match the
// UID type the auth middleware stored, or every lookup reports NeedLoginError.
// The zero value is ready to use, stateless, and safe for concurrent use:
//
//	var auth authorizer.ContextUtils[int64]
//	uid, err := auth.GetCurrentID(ctx)
type ContextUtils[I UID] struct{}

// GetCurrentID returns the UID stored on ctx, or NeedLoginError when no
// Data[I] is present (including when the stored UID type does not match I).
func (ContextUtils[I]) GetCurrentID(ctx context.Context) (I, error) {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		var zeroValue I
		return zeroValue, NeedLoginError
	}
	return data.UID, nil
}

// CheckAuthStatus returns NeedLoginError when no Data[I] is present.
func (c ContextUtils[I]) CheckAuthStatus(ctx context.Context) error {
	_, err := c.GetCurrentID(ctx)
	return err
}

// CheckAuthID verifies that the current user ID matches the provided ID.
// It ensures the user can only access resources belonging to them.
// It returns NeedLoginError when no Data[I] is present and PermissionError
// when the IDs differ.
func (c ContextUtils[I]) CheckAuthID(ctx context.Context, id I) error {
	currentId, err := c.GetCurrentID(ctx)
	if err != nil {
		return err
	}
	if currentId != id {
		return PermissionError
	}
	return nil
}

// GetCurrentSubject returns Data.Subject, or NeedLoginError when no Data[I]
// is present. An authenticated user with no subject yields "".
func (c ContextUtils[I]) GetCurrentSubject(ctx context.Context) (string, error) {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		return "", NeedLoginError
	}
	return data.Subject, nil
}

// GetCurrentRoles returns Data.Roles, or nil when no Data[I] is present.
// An authenticated user with no roles yields a copy of the stored slice, which
// may be empty.
//
// The result is a copy rather than the stored slice: the backing array is
// shared with the identity on the context, so a caller appending to or
// reordering the result would otherwise change the roles that every other
// consumer of the same request — including the permission middleware — sees.
func (c ContextUtils[I]) GetCurrentRoles(ctx context.Context) []string {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		return nil
	}
	return slices.Clone(data.Roles)
}
