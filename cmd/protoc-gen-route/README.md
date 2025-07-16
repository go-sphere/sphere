# protoc-gen-route

`protoc-gen-route` is a protoc plugin that generates routing code from `.proto` files. It is designed to inspect service definitions within your protobuf files and automatically generate corresponding route handlers based on a specified template.


## Installation

To install `protoc-gen-route`, use the following command:

```bash
go install github.com/TBXark/sphere/cmd/protoc-gen-route@latest
```


## Flags

The behavior of `protoc-gen-route` can be customized with the following parameters:

- **`version`**: Print the current plugin version and exit. (Default: `false`)
- **`options_key`**: The key for the option extension in your proto file that contains routing information. (Default: `route`)
- **`file_suffix`**: The suffix for the generated files. (Default: `_route.pb.go`)
- **`template_file`**: Path to a custom Go template file. If not provided, the default internal template is used.
- **`request_model`**: (Required) The fully qualified Go type for the request model (e.g., `github.com/gin-gonic/gin.Context`).
- **`response_model`**: (Required) The fully qualified Go type for the response model.
- **`extra_data_model`**: The fully qualified Go type for an additional data model to be used in the template.
- **`extra_data_constructor`**: A function that constructs and returns a pointer to the `extra_data_model`. (Required if `extra_data_model` is set).


## Usage with Buf

To use `protoc-gen-route` with `buf`, you can configure it in your `buf.gen.yaml` file. Here is an example configuration:

```yaml
version: v2
managed:
  enabled: true
  disable:
    - file_option: go_package_prefix
      module: buf.build/tbxark/options
plugins:
  - local: protoc-gen-go
    out: api
    opt: paths=source_relative
  - local: protoc-gen-route
    out: api
    opt:
      - paths=source_relative
      - options_key=bot
      - file_suffix=_bot.pb.go
      - request_model=github.com/TBXark/sphere/social/telegram;Update
      - response_model=github.com/TBXark/sphere/social/telegram;Message
      - extra_data_model=github.com/TBXark/sphere/social/telegram;MethodExtraData
      - extra_data_constructor=github.com/TBXark/sphere/social/telegram;NewMethodExtraData
```

You will also need to configure the `protoc-gen-route` plugin in your `buf.gen.yaml` so that `buf` knows how to execute it.
