package authorizer

import (
	"context"
)

const (
	// ContextKeyUID is the context key for storing user ID.
	ContextKeyUID = "uid"
	// ContextKeySubject is the context key for storing user subject/username.
	ContextKeySubject = "subject"
	// ContextKeyRoles is the context key for storing user roles.
	ContextKeyRoles = "roles"
)

// ContextUtils provides utility functions for working with authentication context.
// It is parameterized by the user ID type for type safety.
type ContextUtils[I UID] struct{}

// GetCurrentID retrieves the current user ID from the context.
// It returns NeedLoginError if the user is not authenticated or the ID type is incorrect.
func (ContextUtils[I]) GetCurrentID(ctx context.Context) (I, error) {
	var zeroID I
	raw := ctx.Value(ContextKeyUID)
	if raw == nil {
		return zeroID, NeedLoginError
	}
	id, ok := raw.(I)
	if !ok {
		return zeroID, NeedLoginError
	}
	return id, nil
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
func GetCurrentSubject(ctx context.Context) (string, error) {
	raw := ctx.Value(ContextKeySubject)
	username, ok := raw.(string)
	if !ok {
		return "", NeedLoginError
	}
	return username, nil
}

// GetCurrentRoles retrieves the current user's roles from the context.
// It returns nil if no roles are set or if the user is not authenticated.
func GetCurrentRoles(ctx context.Context) []string {
	raw := ctx.Value(ContextKeyRoles)
	if raw == nil {
		return nil
	}
	roles, ok := raw.([]string)
	if !ok {
		return nil
	}
	return roles
}
