# protoc-gen-sphere

`protoc-gen-sphere` is a protoc plugin that generates HTTP server code from `.proto` files. It is designed to inspect
service definitions within your protobuf files and automatically generate corresponding HTTP handlers based on Google
API annotations and a specified template. Inspired
by [protoc-gen-go-http](https://github.com/go-kratos/kratos/tree/main/cmd/protoc-gen-go-http).


## Installation

To install `protoc-gen-sphere`, use the following command:

```bash
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere@latest
```


## Flags

The behavior of `protoc-gen-sphere` can be customized with the following parameters:

- **`version`**: Print the current plugin version and exit. (Default: `false`)
- **`omitempty`**: Omit file generation if `google.api.http` options are not found. (Default: `true`)
- **`omitempty_prefix`**: A file path prefix. If set, `omitempty` will only apply to files with this prefix. (Default: `""`)
- **`template_file`**: Path to a custom Go template file. If not provided, the default internal template is used.
- **`swagger_auth_header`**: The comment for the authorization header in generated Swagger documentation. (Default: `// @Param Authorization header string false "Bearer token"`)
- **`router_type`**: The fully qualified Go type for the router (e.g., `github.com/gin-gonic/gin;IRouter`). (Default: `github.com/gin-gonic/gin;IRouter`)
- **`context_type`**: The fully qualified Go type for the request context (e.g., `github.com/gin-gonic/gin;Context`). (Default: `github.com/gin-gonic/gin;Context`)
- **`data_resp_type`**: The fully qualified Go type for the data response model, which must support generics. (Default: `github.com/TBXark/sphere/server/ginx;DataResponse`)
- **`error_resp_type`**: The fully qualified Go type for the error response model. (Default: `github.com/TBXark/sphere/server/ginx;ErrorResponse`)
- **`server_handler_func`**: The wrapper function for handling server responses. (Default: `github.com/TBXark/sphere/server/ginx;WithJson`)
- **`parse_json_func`**: The function used to parse JSON request bodies. (Default: `github.com/TBXark/sphere/server/ginx;ShouldBindJSON`)
- **`parse_uri_func`**: The function used to parse URI parameters. (Default: `github.com/TBXark/sphere/server/ginx;ShouldBindUri`)
- **`parse_form_func`**: The function used to parse form data/query parameters. (Default: `github.com/TBXark/sphere/server/ginx;ShouldBindQuery`)


## Usage with Buf

To use `protoc-gen-sphere` with `buf`, you can configure it in your `buf.gen.yaml` file. Here is an example configuration:

```yaml
version: v2
managed:
  enabled: true
  disable:
    - file_option: go_package_prefix
      module: buf.build/googleapis/googleapis
    - file_option: go_package_prefix
      module: buf.build/bufbuild/protovalidate
  override:
    - file_option: go_package_prefix
      value: github.com/TBXark/sphere/layout/api
plugins:
  - local: protoc-gen-sphere
    out: api
    opt:
      - paths=source_relative
      - swagger_auth_header=// @Security ApiKeyAuth
```

You will also need to configure the `protoc-gen-sphere` plugin in your `buf.gen.yaml` so that `buf` knows how to execute it.
