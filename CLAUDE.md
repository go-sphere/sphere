# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is the `github.com/go-sphere/sphere` Go module — a library-only collection of building blocks for a Protobuf-first Go service framework. There is no `main` package here; the module is consumed by service projects generated from `sphere-layout`, `sphere-simple-layout`, etc. Code generators (`protoc-gen-sphere*`) and the project scaffolder (`sphere-cli`) live in separate repos under the `go-sphere` org.

## Commands

- `go test ./...` — full test suite; CI runs this on push/PR to `master`.
- `go test ./<pkg>/...` — scope tests to one package (e.g. `go test ./cache/...`). Use this while iterating.
- `go test -run TestName ./<pkg>` — run a single test.
- `make lint` — the authoritative "green" gate. Runs `go fix`, `go fmt`, `go vet`, `go get ./...`, `go test ./...`, `go mod tidy`, then `golangci-lint fmt --no-config --enable gofmt,goimports`, `golangci-lint run --no-config --fix`, and `nilaway -include-pkgs=github.com/go-sphere/sphere ./...`. Run before opening a PR.
- `TAG=v0.0.1 make add-tags` / `make del-tags` — release tag management (signed tags pushed to origin).

`nilaway` is part of the lint gate; new code must keep nil-handling explicit or it will fail there.

## Architecture

### Lifecycle model (`core/task` + `core/boot`)

Everything that runs is modeled as a `task.Task` with `Identifier()`, `Start(ctx)`, `Stop(ctx)`. Composition is done with `task.Group`, which starts members concurrently and cancels the group when any member fails. `boot.Application` is a `Task` wrapping a `Group`; `boot.Run(conf, builder, options...)` wires signal handling, before/after hooks, and a bounded shutdown timeout around it. When adding a new long-running component (HTTP server, MQ consumer, background worker), implement `task.Task` rather than inventing a parallel lifecycle.

`core/safe` provides panic-safe goroutine launchers and deferred cleanup helpers that the task layer relies on; reach for them instead of bare `go func()` in library code.

### Server layer (`server/`)

- `server/httpz` is a thin convention layer on top of `github.com/go-sphere/httpx` (a framework-agnostic HTTP abstraction supporting Gin/Fiber/Echo/Hertz). It defines the standard `DataResponse[T]` / `ErrorResponse` envelopes and the `WithJson` / `WithText` / `WithRecover` handler wrappers. Handlers return `(T, error)`; errors flow through `defaultErrorParser` (overridable via `SetDefaultErrorParser`) and become JSON error responses. New endpoints should be exposed via these wrappers rather than calling `ctx.JSON` directly so the response envelope stays consistent.
- `server/auth/authorizer` is generic over the user-ID type (`UID` constraint: integer or string). `Data[I]` is stored on `context.Context` via a private `authKey` (recently renamed from `contextKey` — see `dfb249d`). Use `authorizer.ContextUtils[I]{}` helpers (`GetCurrentID`, `CheckAuthID`, `GetCurrentSubject`, …) instead of reading the context value directly.
- `server/auth/jwtauth` and `server/auth/acl` plug into that contract; `server/middleware/{auth,cors,online,ratelimiter,selector}` are httpx middlewares built on the same primitives.
- `server/service/{docs,file,reverseproxy}` are reusable mountable services (Swagger docs, file uploader, reverse proxy with cache).

### Pluggable infra interfaces

Each backend-bound package exposes a small generic interface and one or more drivers. When adding a new driver, satisfy the interface — don't widen it.

- `cache` — `Cache[S]` (= `Core[S] + Bulk[S] + TTL[S] + Evictor + io.Closer`), `ExpirableCache[S]`, and aliases `ByteCache` / `ExpirableByteCache`. Drivers: `memory`, `mcache`, `redis`, `badgerdb`, `nscache` (namespaced), `nocache`. `codec_cache.go` and `loader.go` layer serialization and read-through loading on top.
- `mq` — `Queue[T]`, `PubSub[T]`, combined `MessageQueue[T]`. Drivers: `memory`, `redis`.
- `storage` — `Storage` (upload/download/delete/move/copy) and `CDNStorage` (+ `URLHandler` + `UploadAuthorizer` for direct-to-storage uploads). Drivers: `local`, `s3`, `qiniu`. `fileserver` is an HTTP adapter; `urlhandler` and `storageerr` are shared helpers; `kvcache` caches storage metadata.
- `search` — `meilisearch` driver behind `search.go`.
- `log` — `Logger` interface (`BaseLogger + ContextLogger + FormatLogger`) over a `Backend`. Package-level functions (`log.Info`, `log.Errorf`, …) use an `atomic.Pointer[coreLogger]` global initialized to a `StdioBackend`; call `log.InitWithBackends(...)` early in `main` to swap it. `log/zapx` is the zap-backed implementation; structured attrs are constructed with `log.String`, `log.Any`, `log.Err`, etc.

### Test layout

Tests are co-located with source as `_test.go` files. Cross-driver contract tests live under `<pkg>/test/` (e.g. `cache/test/interfaces_test.go`, `mq/test/queue_contract_test.go`, `storage/test/storage_test.go`) and exercise every driver against the same interface; when adding a new driver, register it with these contract tests instead of writing parallel ad-hoc cases. Shared fixtures like an embedded Redis live in `test/redistest`. Prefer table-driven tests with `t.Run` (existing suites in `cache/test` and `utils` are the pattern to follow).

## Conventions

- Single Go module at the repo root; do not introduce nested modules or `go.work`.
- Constructors are `New<Type>`; context is always the first parameter on context-aware funcs.
- Imports are managed by `goimports` via `make lint` — do not hand-order them.
- Commit subjects follow `<type>: <imperative summary>` (`feat`, `fix`, `refactor`, `chore`, …) and stay under ~70 chars. PRs should include the `go test ./...` evidence and call out which top-level packages were touched.
- Timezone defaults to `Asia/Shanghai` via `boot.InitTimezone` in `core/boot/init.go`'s package init; override by calling `InitTimezone` again before `Run`.
