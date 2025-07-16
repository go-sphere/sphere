# protoc-gen-sphere-errors

`protoc-gen-sphere-errors` is a protoc plugin that generates error handling code from `.proto` files. It is designed to inspect service definitions within your protobuf files and automatically generate corresponding error handling code based on a specified template. This code refers to [protoc-gen-go-errors](https://github.com/go-kratos/kratos/tree/main/cmd/protoc-gen-go-errors).


## Installation

To install `protoc-gen-sphere`, use the following command:

```bash
go install github.com/TBXark/sphere/cmd/protoc-gen-go-errors@latest
```


## Usage with Buf

To use `protoc-gen-sphere-errors` with `buf`, you can configure it in your `buf.gen.yaml` file. Here is an example configuration:

```yaml
version: v2
managed:
  enabled: true
  disable:
    - file_option: go_package_prefix
      module: buf.build/tbxark/errors
  override:
    - file_option: go_package_prefix
      value: github.com/TBXark/sphere/layout/api
    - local: protoc-gen-sphere-errors
      out: api
      opt: paths=source_relative
```

You will also need to configure the `protoc-gen-sphere-errors` plugin in your `buf.gen.yaml` so that `buf` knows how to execute it.

