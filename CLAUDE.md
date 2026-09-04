# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Repository Overview

This is the `github.com/go-sphere/sphere` Go module — a library-only collection of building blocks for a Protobuf-first Go service framework. There is no `main` package here; the module is consumed by service projects generated from `sphere-layout`, `sphere-simple-layout`, etc. Code generators (`protoc-gen-sphere*`) and the project scaffolder (`sphere-cli`) live in separate repos under the `go-sphere` org.

## Commands

- `go test ./...` — full test suite; CI runs this on push/PR to `master`.
- `go test ./<pkg>/...` — scope tests to one package (e.g. `go test ./cache/...`). Use this while iterating.
- `go test -run TestName ./<pkg>` — run a single test.
- `make deps-update` — update direct dependencies and tidy the module.
- `make fmt` — format Go code and imports.
- `make lint` — run non-mutating format, vet, golangci-lint, and nilaway checks.
- `make test` — run the full test suite.
- `make check` — the standard green gate: verify dependencies, lint, and test.
- `make verify` — run `make check` plus race-enabled tests.
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
- Imports are managed by `goimports` via `make fmt` — do not hand-order them.
- Commit subjects follow `<type>: <imperative summary>` (`feat`, `fix`, `refactor`, `chore`, …) and stay under ~70 chars. PRs should include the `go test ./...` evidence and call out which top-level packages were touched.
- Timezone defaults to `Asia/Shanghai` via `boot.InitTimezone` in `core/boot/init.go`'s package init; override by calling `InitTimezone` again before `Run`.
