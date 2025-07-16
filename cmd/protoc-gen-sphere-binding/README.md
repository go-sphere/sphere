# protoc-gen-sphere-binding
`protoc-gen-sphere-binding` is a protoc plugin that generates Go struct tags for Sphere binding from `.proto` files. It is designed to inspect service definitions within your protobuf files and automatically generate corresponding Go struct tags based on a specified template. Inspired by [protoc-gen-gotag](https://github.com/srikrsna/protoc-gen-gotag).


## Installation

To install `protoc-gen-sphere`, use the following command:

```bash
go install github.com/TBXark/sphere/cmd/protoc-gen-sphere-binding@latest
```


## Flags
The behavior of `protoc-gen-sphere-binding` can be customized with the following parameters:
- **`version`**: Print the current plugin version and exit. (Default: `false`)
- **`out`**: The output directory for the modified `.proto` files. (Default: `api`)


## Usage with Buf

To use `protoc-gen-sphere-binding` with `buf`, you can configure it in your `buf.binding.yaml` file. `protoc-gen-sphere-binding` can not be used with `buf.gen.yaml` because it does not generate Go code, but rather modifies the `.proto` files to include Sphere binding tags. Here is an example configuration:

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/TBXark/sphere/layout/api
plugins:
  - local: protoc-gen-sphere-binding
    out: api
    opt:
      - paths=source_relative
      - out=api

```


## Example

```protobuf
syntax = "proto3";

import "sphere/binding/binding.proto";

message RunTestRequest {
  option (sphere.binding.default_location) = BINDING_LOCATION_BODY;

  string field_test1 = 1;
  int64 field_test2 = 2;
  string path_test1 = 3 [(sphere.binding.location) = BINDING_LOCATION_URI];
  int64 path_test2 = 4 [(sphere.binding.location) = BINDING_LOCATION_URI];
  string query_test1 = 5 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
  int64 query_test2 = 6 [(sphere.binding.location) = BINDING_LOCATION_QUERY];
}

```