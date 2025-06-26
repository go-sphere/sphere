# API Definition Rules

When defining HTTP transcoding rules in your `.proto` files, Sphere follows specific conventions to map your service methods to RESTful HTTP endpoints. This document outlines the rules for path mapping and request field binding.

## HTTP Method and Field Binding

The binding of request message fields to the HTTP request (URL path, query parameters, or request body) depends on the HTTP method.

1.  **GET / DELETE**: Fields in the request message that are not part of the URL path are automatically treated as URL query parameters.

2.  **POST / PUT / PATCH**:
    *   By default, all fields in the request message that are not bound to the URL path are expected in the JSON request body.
    *   To explicitly bind a field to the URL query parameters instead of the body, you can use a special annotation in the comments of the field.

### Field Binding Annotations

To override the default binding behavior for `POST`, `PUT`, and `PATCH` methods, you can add annotations in the comments
above the field definition in your `.proto` file. The `retags` command recognizes tags based on the `// @sphere:`
prefix.

* `@sphere:uri` or `@sphere:uri="xxx"`: Binds the field to the URL path.
* `@sphere:form` or `@sphere:form="xxx"`: Binds the field to URL query parameters.
* `@sphere:json` or `@sphere:json="xxx"`: Binds the field to the request body.
* `@sphere:!json`: Excludes the field from JSON serialization by adding a `json:"-"` tag.

These annotations allow for fine-grained control over how request data is mapped. The logic for parsing these annotations can be found in `internal/tags/tags.go`.

#### Automatic JSON Omission

By default, when you use `@sphere:uri` or `@sphere:form` to bind a field to the URL path or query parameters, Sphere's
`retags` command will automatically add a `json:"-"` tag to that field. This is a safety feature to prevent fields from
being accidentally exposed in both the URL and the request body.

This behavior is controlled by the `--auto_omit_json` flag in the `sphere-cli retags` command and is enabled by default.

## URL Path Mapping

Sphere converts gRPC-Gateway style URL paths from your `.proto` definitions into Gin-compatible routes. This includes support for path parameters, wildcards, and complex segments.

The following table shows examples of how Protobuf URL paths are translated into Gin routes. This is based on the test cases in `cmd/protoc-gen-sphere/generate/parser/path_test.go`.

| Protobuf Path Template                           | Generated Gin Route                         |
|--------------------------------------------------|---------------------------------------------|
| `/users/{user_id}`                               | `/users/:user_id`                           |
| `/users/{user_id}/posts/{post_id}`               | `/users/:user_id/posts/:post_id`            |
| `/files/{file_path=**}`                          | `/files/*file_path`                         |
| `/files/{name=*}`                                | `/files/:name`                              |
| `/static/{path=assets/*}`                        | `/static/assets/:path`                      |
| `/static/{path=assets/**}`                       | `/static/assets/*path`                      |
| `/projects/{project_id}/locations/{location=**}` | `/projects/:project_id/locations/*location` |
| `/v1/users/{user.id}`                            | `/v1/users/:user_id`                        |
| `/api/{version=v1}/users`                        | `/api/v1/users`                             |
| `/users/{user_id}/posts/{post_id=drafts}`        | `/users/:user_id/posts/drafts`              |
| `/docs/{path=guides/**}`                         | `/docs/guides/*path`                        |
| `users`                                          | `/users`                                    |

### Important Considerations

*   **Routing Conflicts**: Avoid using overly broad wildcard patterns like `/{path_test1:.*}` in path parameters, as this can capture too many routes and lead to unexpected routing behavior.
*   **Body Parsing**: Avoid using `body: "*"` in conjunction with path parameters, as it can cause conflicts and errors during request parsing.

For a complete, practical example, see the [`test.proto`](../proto/shared/v1/test.proto) file and its generated output in [`test_sphere.pb.go`](../api/shared/v1/test_sphere.pb.go).
