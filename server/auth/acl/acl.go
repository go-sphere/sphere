package acl

// ACL represents an Access Control List that manages permissions between subjects and resources.
// It uses a simple allow-based model where permissions must be explicitly granted.
//
// Note: ACL is designed for static configuration during startup (write-once, read-only
// at runtime). If dynamic permission updates are required concurrently at runtime,
// external synchronization or specialized permission management libraries should be used.
// The zero value is an empty ACL ready to use; NewACL is equivalent.
type ACL struct {
	permissions map[string]map[string]bool
}

// NewACL creates a new empty Access Control List.
func NewACL() *ACL {
	return &ACL{
		permissions: make(map[string]map[string]bool),
	}
}

// Allow grants permission for a subject to access a specific resource.
// It creates the permission maps if they don't exist. Granting the
// same pair again is a no-op. Matching is exact and case-sensitive; there are
// no wildcards.
func (a *ACL) Allow(subject, resource string) {
	if a.permissions == nil {
		a.permissions = make(map[string]map[string]bool)
	}
	if _, ok := a.permissions[subject]; !ok {
		a.permissions[subject] = make(map[string]bool)
	}
	a.permissions[subject][resource] = true
}

// IsAllowed reports whether subject may access resource.
// A missing subject or resource is denied.
func (a *ACL) IsAllowed(subject, resource string) bool {
	if subjectPerms, ok := a.permissions[subject]; ok {
		return subjectPerms[resource] // return false if resource not found
	}
	return false
}
