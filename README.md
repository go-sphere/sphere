# Sphere

**Sphere** is a thin integration layer for building Protobuf-first Go services. It is designed as a "frameworkless framework": the stable parts are contracts, adapters, generators, and project conventions; the runtime remains ordinary Go code composed from mature third-party libraries.

Sphere is not meant to replace Go's standard toolchain, Makefiles, Buf, Docker, Ent, Gin, Wire, or other focused tools. Instead, it gives them a consistent place to meet.

## Design Principles

- **Thin core**: keep Sphere small and avoid owning behavior that existing Go libraries already solve well.
- **Replaceable defaults**: templates choose practical defaults, but the defaults are not the framework boundary.
- **Contract-first codegen**: Protobuf files describe service contracts; generators create repeatable plumbing while business code stays handwritten.
- **Makefile as the workflow entrypoint**: generated projects use `make init`, `make gen/*`, `make run`, `make lint`, and `make build` for day-to-day work.
- **No platform lock-in**: deployment, CI, database, frontend client generation, and observability are integrated through templates and recipes, not hidden behind a monolithic CLI.

## What Sphere Provides

### Protoc Code Generation Plugins

- [**`protoc-gen-sphere`**](https://github.com/go-sphere/protoc-gen-sphere) - Generate HTTP handlers, route registration, and Swagger annotations from proto services.
- [**`protoc-gen-route`**](https://github.com/go-sphere/protoc-gen-route) - Generate routing contracts for custom transports and protocol-specific dispatch.
- [**`protoc-gen-sphere-errors`**](https://github.com/go-sphere/protoc-gen-sphere-errors) - Generate typed Go errors from proto enums.
- [**`protoc-gen-sphere-binding`**](https://github.com/go-sphere/protoc-gen-sphere-binding) - Apply request binding tags to generated Go structs.

### Runtime Glue

- [**`httpx`**](https://github.com/go-sphere/httpx) - A small HTTP adapter contract for Gin, Fiber, Echo, Hertz, and similar routers.
- **`server/httpz`** - Response envelopes and handler wrappers on top of `httpx`.
- **`server/middleware`** - Common middleware such as auth, CORS, online tracking, rate limiting, and middleware selectors.
- **`cache`**, **`mq`**, **`scheduler`**, **`storage`** - Interfaces and default adapters for common service infrastructure.
- **`log`**, **`confstore`**, **`infra`** - Practical wrappers for logging, configuration, and infrastructure clients.

### Project Bootstrap and Templates

- [**`sphere-cli`**](https://github.com/go-sphere/sphere-cli) - A small bootstrap tool for creating projects, listing templates, and doing simple generation helpers.
- [**`sphere-layout`**](https://github.com/go-sphere/sphere-layout) - Default template using Ent, Gin, Wire, Buf, Swagger, and TypeScript client generation.
- [**`sphere-simple-layout`**](https://github.com/go-sphere/sphere-simple-layout) - Smaller template for lightweight services.
- [**`sphere-bun-layout`**](https://github.com/go-sphere/sphere-bun-layout) - Template using Bun instead of Ent.

## What Sphere Does Not Try To Own

- Build orchestration: use the template Makefile and normal Go commands.
- Deployment orchestration: use Docker, Compose, Kubernetes, Helm, CI/CD, or your existing platform directly.
- Database modeling: Ent and Bun templates are provided, but Sphere does not require either ORM.
- Full-stack platform behavior: the CLI is intentionally not a replacement for Make, Buf, Docker, or Kubernetes tooling.
- Runtime lock-in: HTTP routers, persistence, queues, storage, and observability should remain replaceable.

## Stable Contracts

Sphere works best when projects keep these boundaries clear:

- `proto/**`: source-of-truth API contracts and codegen metadata.
- `api/**`, `swagger/**`, generated Ent packages: generated artifacts that can be cleaned and regenerated.
- `internal/service/**`: service method implementations that satisfy generated interfaces.
- `internal/biz/**`: domain logic independent from transport and generated code.
- `Makefile`: the project workflow contract.
- `buf.yaml` and `buf.gen.yaml`: Protobuf dependency and generator contract.

## Documentation

For complete documentation, visit [go-sphere.github.io](https://go-sphere.github.io).

- [Quick Start](https://go-sphere.github.io/docs/getting-started)
- [Development Workflow](https://go-sphere.github.io/docs/getting-started/workflow)
- [API Definitions](https://go-sphere.github.io/docs/guides/api-definitions)
- [HTTP Runtime](https://go-sphere.github.io/docs/guides/http-runtime)
- [Error Handling](https://go-sphere.github.io/docs/guides/error-handling)
- [Logging](https://go-sphere.github.io/docs/guides/logging)
- [Cache, Storage, Boot](https://go-sphere.github.io/docs/guides/infrastructure)
- [Upgrading to v0.0.4](https://go-sphere.github.io/docs/guides/upgrading)

## License

MIT License. See [LICENSE](LICENSE) for details.
