package acl

// ACL represents an Access Control List that manages permissions between subjects and resources.
// It uses a simple allow-based model where permissions must be explicitly granted.
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
// It creates the subject's permission map if it doesn't exist.
func (a *ACL) Allow(subject, resource string) {
	if _, ok := a.permissions[subject]; !ok {
		a.permissions[subject] = make(map[string]bool)
	}
	a.permissions[subject][resource] = true
}

// IsAllowed checks if a subject has permission to access a specific resource.
// It returns false if either the subject or resource is not found in the ACL.
func (a *ACL) IsAllowed(subject, resource string) bool {
	if subjectPerms, ok := a.permissions[subject]; ok {
		return subjectPerms[resource] // return false if resource not found
	}
	return false
}
