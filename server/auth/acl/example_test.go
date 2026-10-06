package acl_test

import (
	"fmt"

	"github.com/go-sphere/sphere/server/auth/acl"
)

func ExampleACL() {
	permissions := acl.NewACL()
	permissions.Allow("admin", "orders:write")
	permissions.Allow("admin", "orders:read")
	permissions.Allow("viewer", "orders:read")

	fmt.Println(permissions.IsAllowed("admin", "orders:write"))
	fmt.Println(permissions.IsAllowed("viewer", "orders:read"))
	fmt.Println(permissions.IsAllowed("viewer", "orders:write"))
	fmt.Println(permissions.IsAllowed("guest", "orders:read"))
	// Output:
	// true
	// true
	// false
	// false
}
