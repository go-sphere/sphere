---
name: sphere-framework
description: Sphere monorepo execution guide for AI agents. Use when building or modifying Go backend services with Sphere, protoc plugins, httpx, confstore, entc-extensions, and Sphere layout templates. Covers protocol-first workflow, codegen boundaries, package selection, and troubleshooting.
---

# Sphere Framework Development Guide

## Purpose

This skill turns Sphere tasks into executable steps:

- Identify the correct layer first (protocol/codegen, runtime packages, templates, supporting libraries).
- Execute with the standard workflow (define -> generate -> implement -> validate).
- Strictly separate editable source files from generated artifacts.

## Progressive Disclosure Strategy

Keep this file focused on decisions and guardrails. Load detailed docs from `references/` only when needed:

- Framework overview: `references/00-sphere-overview.md`
- Project setup: `references/01-getting-started.md`
- API development: `references/02-api-development.md`
- Database and ORM: `references/03-database-orm.md`
- Authentication and permissions: `references/04-auth-permissions.md`
- Sphere package selection: `references/05-sphere-packages.md`
- Commands and troubleshooting: `references/99-quick-reference.md`

## Fast Decision Path

1. New project setup -> `01-getting-started.md`
2. Add or change API -> `02-api-development.md`
3. Add schema/model/query -> `03-database-orm.md`
4. Implement auth or permissions -> `04-auth-permissions.md`
5. Choose Sphere runtime package -> `05-sphere-packages.md`
6. Resolve build/runtime issues -> `99-quick-reference.md`

## Parameterized Templates (No Placeholder Style)

Use explicit parameters in commands:

- `<project_name>`: project directory name (for example, `order-service`)
- `<module_path>`: Go module path (for example, `github.com/acme/order-service`)

```bash
sphere-cli create --name <project_name> --module <module_path>
cd <project_name>
make gen/all
make run
```

## Monorepo Component Selection

### Protocol and Codegen

- `protoc-gen-sphere`: proto -> HTTP service code
- `protoc-gen-sphere-binding`: binding tags
- `protoc-gen-sphere-errors`: business error definitions
- `protoc-gen-route`: custom route generation

### Runtime Packages (`sphere/`)

- `cache/*`, `mq/*`, `storage/*`, `log`
- `server/httpz`, `server/middleware/*`, `server/auth/*`
- `core/boot`, `core/task`, `core/pool`, `core/safe`
- `search/*`, `infra/*`, `utils/*`

### Templates and Supporting Libraries

- Templates: `sphere-layout`, `sphere-simple-layout`, `sphere-bun-layout`
- Libraries: `httpx`, `confstore`, `entc-extensions`

## Standard Workflow (Aligned with sphere-layout)

### API changes

```bash
# 1) Edit proto
# 2) Generate
make gen/proto

# 3) Implement service and route registration
# 4) Run
make run
```

### Data model changes

```bash
# 1) Edit ent schema
make gen/db

# 2) Regenerate proto-related code if API is affected
make gen/proto

# 3) Refresh DI
make gen/wire
```

### Full generation

```bash
make gen/all
```

Note: in `sphere-layout`, `gen/proto` depends on `gen/db`. Across different repos/templates, still prefer explicit order: `gen/db -> gen/proto -> gen/wire`.

## Code Boundaries

### Do not edit (generated)

- `api/`
- `internal/pkg/database/ent/`
- `cmd/app/wire_gen.go`
- `swagger/`

### Edit (source/business)

- `proto/**/*.proto`
- `internal/pkg/database/schema/*.go`
- `internal/service/**/*.go`
- `internal/server/**/web.go`
- `*/wire.go`
- runtime config (for example, `config.json`)

## sphere-cli Scope

- Supported: `sphere-cli create`, `sphere-cli rename`
- Limited: `sphere-cli service` only prints template content to stdout

Recommended usage: use `sphere-cli service` for scaffolding text, then create and edit target files manually.

## When Not to Force sphere-layout Paths

If the target repo does not match the standard layout, do not assume fixed paths like:

- `internal/server/api/web.go`
- `internal/pkg/database/schema/`
- `cmd/app/wire.go`

Read the real repository structure first, then map Sphere workflow onto it.

## Troubleshooting Entry

For these issues, go to `references/99-quick-reference.md` first:

- code generators missing
- route 404
- auth 401
- Wire DI errors
- DB/cache connectivity issues

## Documentation Entry Points

- Concepts: `references/00-sphere-overview.md`
- Implementation flow: `references/01-getting-started.md` to `references/04-auth-permissions.md`
- Package selection: `references/05-sphere-packages.md`
- Commands/troubleshooting: `references/99-quick-reference.md`
