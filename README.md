# Sphere

**Sphere** is a multi-server application template. **Sphere** aims to provide a simple, fast, and maintainable multi-server application template. All modules are replaceable, and you can replace modules according to your needs. You can customize your own microservice framework without being limited.


### Features

- **Simple**: Simple code that is easy to maintain.
- **Fast**: One-click code generation for rapid development.
- **Maintainable**: Clear code structure that is easy to extend.
- **Replaceable**: All modules are replaceable.
- **Code Generator**: One-click code generation. The generator automatically generates code, including proto files, Swagger documents, TypeScript clients, and more.


### Core Dependencies

- **Web Framework**: Gin
- **Dependency Injection**: Wire
- **ORM**: Ent


### Usage
```
Usage: make <target>

Targets:
  init                Install all dependencies
  gen-proto           Generate proto files
  gen-docs            Generate swagger docs
  gen-ts              Generate typescript client
  generate            Generate code
  config              Generate config
  dash                Build dash
  build               Build binary
  build-linux-amd     Build linux amd64 binary
  build-linux-arm     Build linux arm64 binary
  build-all           Build all binary
  build-docker        Build docker image
  deploy              Deploy binary
  lint                Run linter
  fmt                 Run formatter
  help                Show this help message
```

### Project Structure

```
├── api                         # generated proto files
├── assets                      # embed assets
├── cmd                         # main entry
├── config                      # configuration
├── devops                      # devops configuration
├── docs                        # documentation generate by swag
├── internal                    # internal packages
│   ├── biz                     # business logic
│   ├── pkg                     # internal common packages
│   └── server                  # server
├── pkg                         # common packages
├── proto                       # proto files
```

### Usage

You can fork this project and modify the code in proto, internal and cmd to implement your own business logic. Please do not modify the code in pkg. If necessary, please raise an issue or PR.

Alternatively, you can import this project in go mod and implement your own business logic in your project.

- Add `ptoto` files in proto directory.
```protobuf
service TestService {
  rpc RunTest(RunTestRequest) returns (RunTestResponse) {
    option (google.api.http) = {
      post: "/api/test/{path_test1}/second/{path_test2}"
      body: "*"
    };
  }
}

message RunTestRequest {
  string field_test1 = 1;
  int64 field_test2 = 2;
  string  path_test1 = 3; // @gotags: json:"-" uri:"path_test1"
  int64  path_test2 = 4; // @gotags: json:"-" uri:"path_test2"
  string  query_test1 = 5; // @gotags: json:"-" form:"query_test1"
  int64  query_test2 = 6; // @gotags: json:"-" form:"query_test2"
}
```
If it is a GET/DELETE request, the non-path field in the request message will be treated as a query parameter.

For other requests, the non-path field in the request message will be treated as a body parameter, unless `@gotags: json:"-" uri:"path_test1"` is added in the message, in which case it will be treated as a path parameter. Or `@gotags: json:"-" form:"query_test1"` is added in the message, in which case it will be treated as a query parameter.

It is not recommended to use pattern matching in the path parameter, such as `/api/test/{path_test1:.*}`, because it will cause routing conflicts. It is also not recommended to use pattern matching in the body parameter, such as `field_test1: .*`, because it will cause parameter parsing errors.

- `make gen-docs` to generate go code and swagger docs.
- Implement the business logic in internal/service.
- Bind service in internal/server.


### License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.