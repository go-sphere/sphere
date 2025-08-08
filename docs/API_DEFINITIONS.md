# Defining HTTP APIs with Protobuf

Sphere allows you to define HTTP interfaces for your services using standard Protobuf definitions and `google.api.http`
annotations. This document outlines the rules and conventions for mapping your gRPC methods to RESTful HTTP endpoints.

## Getting Started: A Basic Example

To expose a gRPC method as an HTTP endpoint, you need to define it in a `.proto` file and add an HTTP annotation.

Here is a basic example of a `TestService` that defines a simple `RunTest` method, exposed as an HTTP `POST` request.

```protobuf
syntax = "proto3";

package your.service.v1;

import "google/api/annotations.proto";
import "sphere/binding/binding.proto";

// The Test service definition.
service TestService {
  // RunTest method
  rpc RunTest(RunTestRequest) returns (RunTestResponse) {
    option (google.api.http) = {
      post: "/v1/test/{path_test1}"
      body: "*"
    };
  }
}

// The request message for the RunTest RPC.
message RunTestRequest {
  // field in request body
  string field_test1 = 1;
  // field in URL path
  string path_test1 = 2 [(sphere.binding.location) = BINDING_LOCATION_URI];
  // field in URL query
  string query_test1 = 3 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
}

// The response message for the RunTest RPC.
message RunTestResponse {
  string field_test1 = 1;
  string path_test1 = 2;
  string query_test1 = 3;
}
```

### Key Components

1. **`import "google/api/annotations.proto";`**: This import is required to use HTTP annotations.
2. **`import "sphere/binding/binding.proto";`**: This import is required for binding annotations.
3. **`service TestService { ... }`**: Defines your gRPC service.
4. **`rpc RunTest(...) returns (...)`**: Defines a method within the service.
5. **`option (google.api.http) = { ... };`**: This is the core of the HTTP mapping.
    * **`post: "/v1/test/{path_test1}"`**: This specifies that the `RunTest` method should be exposed as an HTTP `POST`
      request. The path is `/v1/test/{path_test1}`, where `{path_test1}` is a path parameter.
    * **`body: "*"`**: This specifies that all fields in the `RunTestRequest` message, except those bound to the path,
      should be mapped from the HTTP request body.
6. **`[(sphere.binding.location) = ...]`**: This annotation specifies where the field should be bound from in the HTTP
   request.
    * `BINDING_LOCATION_URI`: Binds the field to a URL path parameter.
    * `BINDING_LOCATION_QUERY`: Binds the field to a URL query parameter.

Sphere uses these definitions to automatically generate server-side stubs and routing information.

## API Definition Rules

When defining HTTP transcoding rules, **Sphere** follows specific conventions to map your service methods to RESTful
HTTP
endpoints.

### URL Path Mapping

Sphere converts gRPC-Gateway style URL paths from your `.proto` definitions into Gin-compatible routes. This includes support for path parameters, wildcards, and complex segments.

The following table shows how Protobuf URL paths are translated into Gin routes.

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

### HTTP Method and Field Binding

The binding of request message fields to the HTTP request (URL path, query parameters, or request body) depends on the
HTTP method.

* **GET / DELETE**: Fields in the request message that are not part of the URL path are automatically treated as URL
  query parameters.

* **POST / PUT / PATCH**:
    * By default (`body: "*"`), all fields in the request message not bound to the URL path are expected in the JSON
      request body.
    * To bind a field to a URL query parameter, you can use the `(sphere.binding.location) = BINDING_LOCATION_QUERY`
      annotation.

### Nested Body and Response

For more complex scenarios, you can specify a single field to be the request body or response body.

```protobuf
syntax = "proto3";

package your.service.v1;

import "google/api/annotations.proto";

service MyService {
  rpc Update(UpdateRequest) returns (UpdateResponse) {
    option (google.api.http) = {
      post: "/v1/items/{item.id}"
      body: "item"
      response_body: "result"
    };
  }
}

message UpdateRequest {
  Item item = 1;
}

message UpdateResponse {
  Item result = 1;
}

message Item {
  string id = 1;
  string name = 2;
}
```

In this example:

- `body: "item"`: Only the `item` field of `UpdateRequest` will be used as the HTTP request body.
- `response_body: "result"`: Only the `result` field of `UpdateResponse` will be sent as the HTTP response body.

### Field Tagging

Sphere's code generator can add struct tags to the generated Go code. This is useful for things like database mapping or
validation.

* **`(sphere.binding.tags)`**: Adds custom tags to a field.
* **`(sphere.binding.default_auto_tags)`**: Sets a default tag key for all fields in a message.

Example:

```protobuf
syntax = "proto3";

package your.service.v1;

import "sphere/binding/binding.proto";

message MyMessage {
  option (sphere.binding.default_auto_tags) = "db";

  string user_id = 1 [(sphere.binding.tags) = "json:"userId""];
  string content = 2;
}
```

This will generate a Go struct similar to this:

```go
type MyMessage struct {
UserId  string `json:"userId" db:"user_id"`
Content string `db:"content"`
}
```

#### Important Considerations

* **Routing Conflicts**: Avoid overly broad wildcard patterns like `/{path_test1:.*}` in path parameters, as this can
  lead to unexpected routing behavior.
* **Body Parsing**: Avoid using `body: "*"` in conjunction with path parameters, as it can cause conflicts during
  request parsing.
* **`oneof` Fields**: Do not use `oneof` in the request or response messages of an RPC if you intend to expose it as an
  HTTP service. The standard JSON codec for Protobuf cannot correctly handle `oneof` fields, which will lead to
  serialization and deserialization errors.

---

For a complete, practical example, see the [`test.proto`](../layout/proto/shared/v1/test.proto) file and its generated output in [`test_sphere.pb.go`](../layout/api/shared/v1/test_sphere.pb.go).
