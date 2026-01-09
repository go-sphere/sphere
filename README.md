# Sphere

**Sphere** is a Protobuf-first Go service framework for definition-driven development. Define once, generate rapidly, and scale seamlessly from monolithic architecture to microservices.

## Core Components

### Protoc Code Generation Plugins

- [**`protoc-gen-sphere`**](https://github.com/go-sphere/protoc-gen-sphere) - Define HTTP services using proto3 and automatically generate Go server code
- [**`protoc-gen-route`**](https://github.com/go-sphere/protoc-gen-route) - Generate routing code for generic servers
- [**`protoc-gen-sphere-errors`**](https://github.com/go-sphere/protoc-gen-sphere-errors) - Define error types in proto3 and automatically generate error handling code
- [**`protoc-gen-sphere-binding`**](https://github.com/go-sphere/protoc-gen-sphere-binding) - Generate custom binding tags for Go structs (overcome proto3 limitations)

### HTTP Framework

- [**`httpx`**](./httpx) - Unified HTTP framework abstraction supporting popular Go HTTP frameworks (Gin, Fiber, Echo, Hertz, etc.)

### Tools and Libraries

- [**`sphere-cli`**](https://github.com/go-sphere/sphere-cli) - Project generation and management tool
- [**`confstore`**](./confstore) - Configuration management library
- [**`entc-extensions`**](./entc-extensions) - Ent ORM enhancement tools

## Project Templates

- [**`sphere-layout`**](https://github.com/go-sphere/sphere-layout) - Default template with Ent as ORM
- [**`sphere-simple-layout`**](https://github.com/go-sphere/sphere-simple-layout) - Simplified version
- [**`sphere-bun-layout`**](https://github.com/go-sphere/sphere-bun-layout) - Template with Bun as ORM

## Documentation

For complete documentation, visit [go-sphere.github.io](https://go-sphere.github.io)

- [Quick Start](https://go-sphere.github.io/docs/getting-started)
- [API Definitions](https://go-sphere.github.io/docs/guides/api-definitions)
- [Error Handling](https://go-sphere.github.io/docs/guides/error-handling)
- [Logging](https://go-sphere.github.io/docs/guides/logging)

## License

MIT License. See [LICENSE](LICENSE) for details.