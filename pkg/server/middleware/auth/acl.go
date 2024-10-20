package auth

type ACL struct {
	permissions map[string]map[string]bool
}

func NewACL() *ACL {
	return &ACL{
		permissions: make(map[string]map[string]bool),
	}
}

func (a *ACL) Allow(subject, resource string) {
	if _, ok := a.permissions[subject]; !ok {
		a.permissions[subject] = make(map[string]bool)
	}
	a.permissions[subject][resource] = true
}

func (a *ACL) IsAllowed(subject, resource string) bool {
	if subjectPerms, ok := a.permissions[subject]; ok {
		return subjectPerms[resource] // return false if resource not found
	}
	return false
}
