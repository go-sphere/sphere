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

- **Web Framework**: Gin
- **Dependency Injection**: Wire
- **ORM**: Ent

## Getting Started

Use the [`sphere-cli`](cmd/sphere-cli/README.md) tool to generate a new project from the [standard layout](./layout/README.md).

## License

**Sphere**  is released under the MIT license. See [LICENSE](LICENSE) for details.