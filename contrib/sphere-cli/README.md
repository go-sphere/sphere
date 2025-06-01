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

### `sphere-cli create`

```
Usage:
  sphere-cli create [flags]

Flags:
  -h, --help            help for create
  -m, --module string   Go module name for the project (optional)
  -n, --name string     Name of the new Sphere project
```

### `sphere-cli ent2proto`

```
Convert Ent schema to Protobuf definitions, generating .proto files from Ent schema definitions.

Usage:
  sphere-cli ent2proto [flags]

Flags:
  -a, --all-fields-required             Treat all fields as required in Protobuf (default true)
  -A, --auto-add-annotation             Automatically add annotations to the schema (default true)
  -e, --enum-use-raw-type               Use raw type for enums in Protobuf (default true)
  -h, --help                            help for ent2proto
  -i, --import-proto string             Import Protobuf definitions, format: path1,package1,type1,type2;path2,package2,type3,type4; (default "google/protobuf/any.proto,google.protobuf,Any;")
  -p, --proto string                    Path to the output Protobuf directory (default "./proto")
  -s, --schema string                   Path to the Ent schema directory (default "./schema")
  -k, --skip-unsupported                Skip unsupported types in Protobuf generation (default true)
  -t, --time-proto-type string          Protobuf type for time.Time (options: int64, string, google.protobuf.Timestamp) (default "int64")
  -x, --unsupported-proto-type string   Protobuf type for unsupported types (options: google.protobuf.Any, google.protobuf.Struct, bytes) (default "google.protobuf.Any")
  -u, --uuid-proto-type string          Protobuf type for uuid.UUID (options: string, bytes) (default "string")
```

### `sphere-cli rename`

```
Rename the Go module in the specified directory from old to new name.

Usage:
  sphere-cli rename [flags]

Flags:
  -h, --help         help for rename
  -n, --new string   New Go module name
  -o, --old string   Old Go module name
```

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