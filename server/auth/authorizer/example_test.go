package authorizer_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/server/auth/authorizer"
)

func ExampleContextUtils() {
	var auth authorizer.ContextUtils[int64]

	// Auth middleware stores the identity; handlers only read it.
	ctx := authorizer.WithAuthData(context.Background(), authorizer.Data[int64]{
		UID:     42,
		Subject: "ada",
		Roles:   []string{"admin"},
	})

	uid, err := auth.GetCurrentID(ctx)
	fmt.Println(uid, err)
	fmt.Println(auth.CheckAuthID(ctx, 42))
	fmt.Println(errors.Is(auth.CheckAuthID(ctx, 7), authorizer.PermissionError))
	fmt.Println(auth.GetCurrentRoles(ctx))
	// Output:
	// 42 <nil>
	// <nil>
	// true
	// [admin]
}

func ExampleContextUtils_GetCurrentID_unauthenticated() {
	var auth authorizer.ContextUtils[int64]

	_, err := auth.GetCurrentID(context.Background())
	fmt.Println(errors.Is(err, authorizer.NeedLoginError))

	// Data stored under a different UID type is not visible.
	ctx := authorizer.WithAuthData(context.Background(), authorizer.Data[string]{UID: "42"})
	_, err = auth.GetCurrentID(ctx)
	fmt.Println(errors.Is(err, authorizer.NeedLoginError))
	// Output:
	// true
	// true
}

type apiKeyClaims struct{ uid string }

func (c apiKeyClaims) GetUID() (string, error)     { return c.uid, nil }
func (c apiKeyClaims) GetSubject() (string, error) { return "", nil }
func (c apiKeyClaims) GetRoles() ([]string, error) { return nil, nil }

func ExampleParserFunc() {
	keys := map[string]string{"key-123": "service-a"}
	var parser authorizer.Parser[string, apiKeyClaims] = authorizer.ParserFunc[string, apiKeyClaims](
		func(ctx context.Context, token string) (apiKeyClaims, error) {
			uid, ok := keys[token]
			if !ok {
				return apiKeyClaims{}, authorizer.TokenNotFoundError
			}
			return apiKeyClaims{uid: uid}, nil
		},
	)

	claims, err := parser.ParseToken(context.Background(), "key-123")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	uid, err := claims.GetUID()
	fmt.Println(uid, err)
	// Output:
	// service-a <nil>
}
