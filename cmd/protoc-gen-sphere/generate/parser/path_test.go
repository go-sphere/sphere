package parser

import "testing"

func TestGinURIParams(t *testing.T) {
	testCases := [][2]string{
		{"/users/{user_id}", "/users/:user_id"},
		{"/users/{user_id}/posts/{post_id}", "/users/:user_id/posts/:post_id"},
		{"/files/{file_path=**}", "/files/*file_path"},
		{"/files/{name=*}", "/files/:name"},
		{"/static/{path=assets/*}", "/static/assets/:path"},
		{"/static/{path=assets/**}", "/static/assets/*path"},
		{"/projects/{project_id}/locations/{location=**}", "/projects/:project_id/locations/*location"},
		{"/v1/users/{user.id}", "/v1/users/:user_id"},
		{"/api/{version=v1}/users", "/api/v1/users"},
		{"/users/{user_id}/posts/{post_id=drafts}", "/users/:user_id/posts/drafts"},
		{"/docs/{path=guides/**}", "/docs/guides/*path"},
		{"users", "/users"},
	}
	for _, tc := range testCases {
		protoPath, expectedGinPath := tc[0], tc[1]
		ginPath, err := GinRoute(protoPath)
		if err != nil {
			t.Errorf("GinRoute(%q) returned error: %v", protoPath, err)
			continue
		}
		if ginPath != expectedGinPath {
			t.Errorf("GinRoute(%q) = %q; want %q", protoPath, ginPath, expectedGinPath)
			continue
		}
		t.Logf("%q\n%q\n%v", protoPath, ginPath, GinURIParams(ginPath))
	}
}
