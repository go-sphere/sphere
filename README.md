# Sphere

**Sphere** is a Protobuf-first Go service framework for definition-driven development. Start modular monolithic, scale to microservices. Define once, generate everything with rapid tooling.

Sphere comes with powerful code generation tools to create `proto` files, `Swagger` documents, `TypeScript` clients, and
more, speeding up your development workflow.

## Features

- **Protocol-First Design**

  Define once in Protobuf, generate everywhere. Get Go handlers, HTTP routing, client SDKs,
  and OpenAPI docs from a single source of truth.

- **Pragmatic Monolith Template**

  Start simple with `sphere layout` in a single binary. Clean architecture that scales
  from MVP to microservices when needed.
  
- **Complete Code Generation**

  Automated toolchain with protoc-gen-sphere ecosystem: server stubs, HTTP routing, field
  binding, typed errors, and validation.

- **Structured Error Handling**

  Define error enums in protobuf with automatic HTTP status mapping. Get consistent JSON
  responses with code, reason, and message.

- **Full-Stack Development**

  Generate Swagger documentation, TypeScript SDKs, and validation schemas. Bridge backend
  and frontend with type safety.

- **Developer Experience**
  
  sphere-cli for project scaffolding, Makefile workflows, and clean project structure. Focus
  on business logic, not boilerplate.

## Command line Tool

- [**`sphere-cli`**](https://github.com/go-sphere/sphere-cli) : A command-line tool for `sphere` project management.
- [**`protoc-gen-route`**](https://github.com/go-sphere/protoc-gen-route) : A plugin for generating routing code from `.proto` files.
- [**`protoc-gen-sphere`**](https://github.com/go-sphere/protoc-gen-sphere) : A plugin for generating HTTP server code from `.proto` files.
- [**`protoc-gen-sphere-binding`**](https://github.com/go-sphere/protoc-gen-sphere-binding) : A plugin for replacing go struct binding tags
  with `proto` field options.
- [**`protoc-gen-sphere-errors`**](https://github.com/go-sphere/protoc-gen-sphere-errors) : A plugin for generating error handling code
  from `.proto` files.

## Layout template

- [**`sphere-layout`**](https://github.com/go-sphere/sphere-layout) : Default sphere project layout template with `ent`
  as ORM.
- [**`sphere-simple-layout`**](https://github.com/go-sphere/sphere-simple-layout) : A simplified version of the Sphere
  project layout template.
- [**`sphere-bun-layout`**](https://github.com/go-sphere/sphere-bun-layout) : A layout template with `bun` as ORM.

## Documentation

- [**Quick Start Guide**](https://go-sphere.github.io/docs/getting-started) : A step-by-step guide to setting up a new Sphere project.
- [**API Definitions**](https://go-sphere.github.io/docs/guides/api-definitions) : Guidelines for writing API definitions `.proto` files in Sphere.
- [**Error Handling**](https://go-sphere.github.io/docs/guides/error-handling) : Guidelines for error handling in Sphere applications.
- [**Logging**](https://go-sphere.github.io/docs/guides/logging) : How to set up and use logging in Sphere applications.

## Core Dependencies

- **Dependency Injection**: [wire](https://github.com/google/wire)
- **ORM**: [ent](https://github.com/ent/ent) or [bun](https://github.com/uptrace/bun)
- **Docs Generation**: [swag](https://github.com/swaggo/swag)
- **Protobuf management**: [buf](https://github.com/bufbuild/buf)
- **Build Tool**: make

## License

**Sphere** is released under the MIT license. See [LICENSE](LICENSE) for details.