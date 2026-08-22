package acl

import "testing"

// TestAllowThenCheck pins the allow-based model: permission exists only after an
// explicit Allow, and granting one resource must not grant another.
func TestAllowThenCheck(t *testing.T) {
	a := NewACL()

	if a.IsAllowed("alice", "users") {
		t.Fatal("a fresh ACL must deny everything")
	}

	a.Allow("alice", "users")

	if !a.IsAllowed("alice", "users") {
		t.Fatal("an explicitly granted permission must pass")
	}
	if a.IsAllowed("alice", "billing") {
		t.Fatal("granting one resource must not grant another")
	}
	if a.IsAllowed("bob", "users") {
		t.Fatal("granting one subject must not grant another")
	}
}

// TestUnknownEntriesAreDenied pins the fail-closed lookup for subjects and
// resources that were never mentioned.
func TestUnknownEntriesAreDenied(t *testing.T) {
	a := NewACL()
	a.Allow("alice", "users")

	for _, tc := range []struct {
		subject  string
		resource string
	}{
		{subject: "missing-subject", resource: "users"},
		{subject: "alice", resource: "missing-resource"},
		{subject: "missing-subject", resource: "missing-resource"},
	} {
		if a.IsAllowed(tc.subject, tc.resource) {
			t.Errorf("IsAllowed(%q, %q) = true, want false", tc.subject, tc.resource)
		}
	}
}

// TestAllowIsIdempotent pins that granting twice neither fails nor widens the
// grant.
func TestAllowIsIdempotent(t *testing.T) {
	a := NewACL()
	a.Allow("alice", "users")
	a.Allow("alice", "users")

	if !a.IsAllowed("alice", "users") {
		t.Fatal("the grant was lost on re-allow")
	}
}
