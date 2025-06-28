# Sphere

> This project is under active development. APIs may change at any time. Please use it with caution.

**Sphere** is a project template for building monolithic applications with a focus on simplicity, maintainability, and scalability. It uses `ent` for schema management and `proto` for API definitions, providing a solid foundation that can be adapted for microservices as your project evolves.

Sphere comes with powerful code generation tools to create `proto` files, `Swagger` documents, `TypeScript` clients, and more, accelerating your development workflow.

## Features

- **Simple & Maintainable**: A clean and straightforward codebase that is easy to understand and extend.
- **Rapid Development**: Use the code generator to quickly scaffold project components.
- **Modular Design**: All modules are designed to be replaceable to fit your specific needs.
- **Single-File Deployment**: The entire project can be deployed as a single file for easy management.

## Core Dependencies

- **Web Framework**: [gin](https://gin-gonic.com)
- **Dependency Injection**: [wire](https://github.com/google/wire)
- **ORM**: [ent](https://entgo.io)

## Core Tools

- **Code Generation**: [ent](https://entgo.io), [swag](https://github.com/swaggo/swag), [sphere](cmd/README.md)
- **Protobuf management**: [buf](https://buf.build)
- **Build Tool**: make

## Command line Tool

- [`sphere-cli`](cmd/sphere-cli/README.md) - A command-line tool for `sphere` project management.
- [`protoc-gen-route`](cmd/protoc-gen-route/README.md) - A plugin for generating routing code from `.proto` files.
- [`protoc-gen-sphere`](cmd/protoc-gen-sphere/README.md) - A plugin for generating HTTP server code from `.proto` files.
- [`protoc-gen-sphere-errors`](cmd/protoc-gen-sphere-errors/README.md) - A plugin for generating error handling code from `.proto` files.

## Documentation

- [Standard Layout](./layout/README.md). - Default sphere project template layout.
- [Quick Start Guide](./layout/docs/QUICK_START.md) - A step-by-step guide to setting up a new Sphere project.
- [API Definitions](./layout/docs/API_DEFINITIONS.md) - Guidelines for writing API definitions `.proto` files in Sphere.
- [Error Handling](./layout/docs/ERROR_HANDLING.md) - Guidelines for error handling in Sphere applications.

## License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.