# sphere-cli

This is a cli tool to create a new sphere project, and generate code for Sphere.

```
sphere-cli
Usage:
  sphere-cli [flags]
  sphere-cli [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  create      Create a new Sphere project
  ent2proto   Convert Ent schema to Protobuf definitions
  help        Help about any command
  rename      Rename Go module in a directory
  service     Generate service code

Flags:
  -h, --help   help for sphere-cli

Use "sphere-cli [command] --help" for more information about a command.
```

---

### `sphere-cli create`

```
Create a new Sphere project with the specified name and optional template.

Usage:
  sphere-cli create [flags]

Flags:
  -h, --help            help for create
      --module string   Go module name for the project (optional)
      --name string     Name of the new Sphere project
```

---

### `sphere-cli ent2proto`

```
Convert Ent schema to Protobuf definitions, generating .proto files from Ent schema definitions.

Usage:
  sphere-cli ent2proto [flags]

Flags:
      --all_fields_required             ignore optional, use zero value instead (default true)
      --auto_annotation                 auto add annotation to the schema (default true)
      --enum_raw_type                   use string for enum (default true)
  -h, --help                            help for ent2proto
      --import_proto string             import proto, format: path1,package1,type1,type2;path2,package2,type3,type4; (default "google/protobuf/any.proto,google.protobuf,Any;")
      --path string                     path to schema directory (default "./schema")
      --proto string                    path to proto directory (default "./proto")
      --skip_unsupported                skip unsupported types, when unsupportedProtoType is not set (default true)
      --time_proto_type string          use proto type for time.Time, one of int64, string, google.protobuf.Timestamp (default "int64")
      --unsupported_proto_type string   use proto type for unsupported types, one of google.protobuf.Any, google.protobuf.Struct, bytes (default "google.protobuf.Any")
      --uuid_proto_type string          use proto type for uuid.UUID, one of string, bytes (default "string")
```

---

### `sphere-cli rename`

```
Rename the Go module in the specified directory from old to new name.

Usage:
  sphere-cli rename [flags]

Flags:
  -h, --help            help for rename
      --new string      New Go module name
      --old string      Old Go module name
      --target string   Target directory to rename the module in (default ".")
```

---

### `sphere-cli retags`

```
Refer to "favadi/protoc-go-inject-tag", which is specifically optimized for the sphere project.

Usage:
  sphere-cli retags [flags]

Flags:
  -h, --help                 help for retags
      --input string         pattern to match input file(s) (default "./api/*/*/*.pb.go")
      --remove_tag_comment   remove tag comment (default true)
```

---

### `sphere-cli service`

```
Generate service code for Sphere projects, including service interfaces and implementations.

Usage:
  sphere-cli service [command]

Available Commands:
  golang      Generate service Golang code
  proto       Generate service proto code

Flags:
  -h, --help   help for service

Use "sphere-cli service [command] --help" for more information about a command.
```

#### `sphere-cli service golang`

```
Generate service Golang code for Sphere projects, including service interfaces and implementations in Go.

Usage:
  sphere-cli service golang [flags]

Flags:
  -h, --help             help for golang
      --mod string       Go module path for the generated code (default "github.com/TBXark/sphere/layout")
      --name string      Name of the service
      --package string   Package name for the generated Go code (default "dash.v1")
```

#### `sphere-cli service proto`

```
Generate service proto code for Sphere projects, including proto definitions and gRPC service implementations.

Usage:
  sphere-cli service proto [flags]

Flags:
  -h, --help             help for proto
      --name string      Name of the service
      --package string   Package name for the generated proto code (default "dash.v1")
```