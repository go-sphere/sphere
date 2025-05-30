# Sphere project layout

## About

`Sphere` is a server scaffolding that uses `ent` as the database structure definition and `proto` as the interface definition. It also provides a series of code and document generation tools, including `proto` files, `Swagger` documents, `TypeScript` clients, etc.

You can fork this project and modify the code in proto, internal and cmd to implement your own business logic. Please do not modify the code in pkg. If necessary, please raise an issue or PR.

Alternatively, you can import this project in go mod and implement your own business logic in your project.



## Usage
```
Sphere build tool. Usage: make [target]

  build                Build binary
  build/linux/amd64    Build linux amd64 binary
  build/linux/arm64    Build linux arm64 binary
  build/all            Build all arch binary
  build/assets         Build assets
  gen/dts              Generate swagger typescript docs
  gen/proto            Generate proto files and run protoc plugins
  gen/db               Generate ent code
  gen/docs             Generate swagger docs
  gen/wire             Generate wire code
  gen/conf             Generate example config
  clean                Clean gen code and build files
  gen/all              Generate both ent, docs and wire
  build/docker         Build docker image
  build/multi-docker   Build multi-arch docker image
  run                  Run the application
  deploy               Deploy binary
  install              Install dependencies tools
  lint                 Run linter
  fmt                  Run formatter
  init                 Init all dependencies
  help                 Show this help message
```

## Project Structure

```
├── api                         # generated go files by protoc
├── assets                      # embed assets
├── cmd                         # main entry
├── devops                      # devops configuration
├── internal                    # internal packages
│   ├── biz                     # business logic
│   ├── config                  # configuration
│   ├── pkg                     # internal common packages
│   └── server                  # server
├── pkg                         # common packages
├── proto                       # proto files
├── swagger                     # documentation generate by swag
```


## Quick Start

### 1. Create a new project with `sphere-cli`

Although you use `sphere-cli` to create the project, you are free to modify the project structure subsequently. `sphere` just provides a template, and you are free to modify the project structure according to your own needs. This way you can freely develop your own projects without relying on any framework.

```bash
go install github.com/TBXark/sphere/contrib/sphere-cli@latest
sphere-cli create -name myproject -mod github.com/TBXark/myproject
```


### 2. Define the database structure

You can define the database structure in the `/internal/database/ent/schema` directory. For details, please refer to the [ent documentation](https://entgo.io/docs/getting-started).

When you have finished defining the database structure, you can run the following command to generate the database structure code and proto files.

```bash
make gen-ent
```


### 3. Define the http server interface

You can define the http server interface in the `/proto` directory. For details, please refer to the [proto documentation](https://developers.google.com/protocol-buffers/docs/proto3).

When you have finished defining the http server interface, you can run the following command to generate the http server code and swagger docs.

```bash
make gen-docs
```

There are some rules for defining the http server interface:

1. If it is a GET/DELETE request, the non-path field in the request message will be treated as a query parameter.
2. For other requests, the non-path field in the request message will be treated as a body parameter, unless `@gotags: json:"-" uri:"path_test1"` is added in the message, in which case it will be treated as a path parameter. Or `@gotags: json:"-" form:"query_test1"` is added in the message, in which case it will be treated as a query parameter.
3. It is not recommended to use pattern matching in the path parameter, such as `/api/test/{path_test1:.*}`, because it will cause routing conflicts. It is also not recommended to use pattern matching in the body parameter, such as `field_test1: .*`, because it will cause parameter parsing errors.


### 4. Implement the business logic

You can implement the business logic in the `/internal` directory. And bind the business logic to the http server.


### 5. Start the server

Add entry in `cmd` directory, You can use `wire` to inject dependencies. For details, please refer to the [wire documentation](https://github.com/google/wire)