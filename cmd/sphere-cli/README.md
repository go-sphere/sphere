# Sphere CLI

Sphere CLI (`sphere-cli`) is a command-line tool designed to streamline the development of [Sphere](https://github.com/TBXark/sphere) projects. It helps you create new projects, generate service code, manage Protobuf definitions, and perform other common development tasks.


## Installation

To install `sphere-cli`, ensure you have Go installed and run the following command:

```shell
go install github.com/TBXark/sphere/cmd/sphere-cli@latest
```


## Usage

The general syntax for `sphere-cli` is:

```shell
sphere-cli [command] [flags]
```

For detailed information on any command, you can use the `--help` flag:

```shell
sphere-cli [command] --help
```


## Commands

Here is an overview of the available commands.

---

### `create`

Initializes a new Sphere project with a default template.

**Usage:**
```shell
sphere-cli create --name <project-name> [--module <go-module-name>]
```

**Flags:**
- `--name string`: (Required) The name for the new Sphere project.
- `--module string`: (Optional) The Go module path for the project.

---

### `entproto`

Converts Ent schemas into Protobuf (`.proto`) definitions. This command reads your Ent schema files and generates corresponding `.proto` files.

**Usage:**
```shell
sphere-cli entproto [flags]
```

**Flags:**
- `--path string`: Path to the Ent schema directory (default: `./schema`).
- `--proto string`: Output directory for the generated `.proto` files (default: `./proto`).
- `--all_fields_required`: Treat all fields as required, ignoring `Optional()` (default: `true`).
- `--auto_annotation`: Automatically add `@entproto` annotations to the schema (default: `true`).
- `--enum_raw_type`: Use `string` as the type for enums in Protobuf (default: `true`).
- `--skip_unsupported`: Skip fields with types that are not supported (default: `true`).
- `--time_proto_type string`: Protobuf type to use for `time.Time` fields. Options: `int64`, `string`, `google.protobuf.Timestamp` (default: `int64`).
- `--uuid_proto_type string`: Protobuf type to use for `uuid.UUID` fields. Options: `string`, `bytes` (default: `string`).
- `--unsupported_proto_type string`: Protobuf type to use for unsupported fields. Options: `google.protobuf.Any`, `google.protobuf.Struct`, `bytes` (default: `google.protobuf.Any`).
- `--import_proto string`: Define external Protobuf imports. Format: `path1,package1,type1;path2,package2,type2` (default: `google/protobuf/any.proto,google.protobuf,Any;`).

---

### `service`

Generates service code, including both Protobuf definitions and Go service implementations.

This command has two subcommands: `proto` and `golang`.

#### `service proto`

Generates a `.proto` file for a new service.

**Usage:**
```shell
sphere-cli service proto --name <service-name> [--package <package-name>]
```

**Flags:**
- `--name string`: (Required) The name of the service.
- `--package string`: The package name for the generated `.proto` file (default: `dash.v1`).

#### `service golang`

Generates the Go implementation for a service from its definition.

**Usage:**
```shell
sphere-cli service golang --name <service-name> [--package <package-name>] [--mod <go-module-path>]
```

**Flags:**
- `--name string`: (Required) The name of the service.
- `--package string`: The package name for the generated Go code (default: `dash.v1`).
- `--mod string`: The Go module path for the generated code (default: `github.com/TBXark/sphere/layout`).

---

### `retags`

Injects struct tags into generated Protobuf message files (`.pb.go`). This command is an optimization for the Sphere framework, inspired by `favadi/protoc-go-inject-tag`.

It supports special `// @sphere:` comments to inject tags. For example, a comment `// @sphere:json="name"` on a field
will add the struct tag ``json:"name"``.

A special annotation, `// @sphere:!json`, can be used to explicitly exclude a field from JSON serialization by adding
the `json:"-"` tag.

**Usage:**
```shell
sphere-cli retags [--input <glob-pattern>]
```

**Flags:**
- `--input string`: Glob pattern to find target `.pb.go` files (default: `./api/*/*/*.pb.go`).
- `--remove_tag_comment`: Remove tag comments after injection (default: `true`).
- `--auto_omit_json`: Automatically add `json:"-"` for fields that have `form` or `uri` tags. This helps prevent
  accidental exposure of fields in the request body when they are already bound to the URL path or query string (
  default: `true`).

---

### `rename`

Performs a project-wide rename of the Go module path.

**Usage:**
```shell
sphere-cli rename --old <old-module> --new <new-module> [--target <directory>]
```

**Flags:**
- `--old string`: (Required) The current Go module name.
- `--new string`: (Required) The new Go module name.
- `--target string`: The root directory of the project to rename (default: `.`).

---

### Other Commands

- `completion`: Generates shell autocompletion scripts (for Bash, Zsh, etc.).
- `help`: Provides help for any command.
