# Sphere

> This project is under development. APIs may change at any time. Please use it with caution.

**Sphere** is a Monolithic Architecture (MA) application template that can be used to develop microservices.
It is designed to be a simple, fast, and maintainable codebase that can be easily extended and replaced.
Sphere uses `ent` as the database structure definition and `proto` as the interface definition.
It also provides a series of code and document generation tools, including `proto` files, `Swagger` documents,
`TypeScript` clients, etc.

## Features

- **Simple**: Simple code that is easy to maintain.
- **Fast**: One-click code generation for rapid development.
- **Maintainable**: Clear code structure that is easy to extend.
- **Replaceable**: All modules are replaceable.
- **Code Generator**: One-click code generation. The generator automatically generates code, including proto files, Swagger documents, TypeScript clients, and more.
- **Deployment By One file**: The entire project can be deployed with a single file, making it easy to deploy and
  manage.


## Core Dependencies

- **Web Framework**: Gin
- **Dependency Injection**: Wire
- **ORM**: Ent

## Getting Started

Use the [`sphere-cli`](./contrib/sphere-cli/README.md) command-line tool to generate a project with
the [standard layout](./layout/README.md).
Full standard layout documentation is available in the [layout](./layout/README.md) directory.

## License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.