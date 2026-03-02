package authorizer

import (
	"context"
)

type authKey struct{}

var authContextKey = authKey{}

type Data[I UID] struct {
	UID     I        `json:"uid"`
	Subject string   `json:"subject"`
	Roles   []string `json:"roles"`
}

func WithAuthData[I UID](ctx context.Context, data Data[I]) context.Context {
	return context.WithValue(ctx, authContextKey, data)
}

func GetAuthData[I UID](ctx context.Context) (Data[I], bool) {
	raw := ctx.Value(authContextKey)
	if raw == nil {
		return Data[I]{}, false
	}
	data, ok := raw.(Data[I])
	if !ok {
		return Data[I]{}, false
	}
	return data, true
}

// ContextUtils provides utility functions for working with authentication context.
// It is parameterized by the user ID type for type safety.
type ContextUtils[I UID] struct{}

// GetCurrentID retrieves the current user ID from the context.
// It returns NeedLoginError if the user is not authenticated or the ID type is incorrect.
func (ContextUtils[I]) GetCurrentID(ctx context.Context) (I, error) {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		var zeroValue I
		return zeroValue, NeedLoginError
	}
	return data.UID, nil
}

// CheckAuthStatus verifies that the user is authenticated.
// It returns an error if authentication is required but not present.
func (c ContextUtils[I]) CheckAuthStatus(ctx context.Context) error {
	_, err := c.GetCurrentID(ctx)
	return err
}

// CheckAuthID verifies that the current user ID matches the provided ID.
// It ensures the user can only access resources belonging to them.
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

// GetCurrentSubject retrieves the current user's subject/username from the context.
// It returns NeedLoginError if the user is not authenticated.
func (c ContextUtils[I]) GetCurrentSubject(ctx context.Context) (string, error) {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		return "", NeedLoginError
	}
	return data.Subject, nil
}

// GetCurrentRoles retrieves the current user's roles from the context.
// It returns nil if no roles are set or if the user is not authenticated.
func (c ContextUtils[I]) GetCurrentRoles(ctx context.Context) []string {
	data, ok := GetAuthData[I](ctx)
	if !ok {
		return nil
	}
	return data.Roles
}
