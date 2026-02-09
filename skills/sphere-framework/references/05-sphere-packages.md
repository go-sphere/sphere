# 05 - Sphere Packages

> **AI Agent Context**
> Read this document when you need to choose Sphere runtime packages (cache, auth, queue, storage, lifecycle, etc.). The goal is to quickly decide which package to use, where it fits in layout projects, and what pitfalls to avoid.

## Table of Contents

1. Package group overview
2. Group details (purpose/entry points/dependencies/pitfalls)
3. Mapping to sphere-layout
4. Minimal runnable example index

## 1) Package Group Overview

| Group | Representative Packages | Primary Use |
|---|---|---|
| core | `core/boot` `core/task` `core/pool` `core/safe` | app lifecycle, task orchestration, pooling, safe execution |
| server | `server/httpz` `server/middleware/*` `server/auth/*` | HTTP wrappers, middleware, authentication/authorization |
| cache | `cache` `cache/redis` `cache/memory` `cache/badgerdb` | unified cache abstractions with multiple backends |
| mq | `mq` `mq/redis` `mq/memory` | queue and pub/sub messaging |
| storage | `storage` `storage/local` `storage/s3` `storage/qiniu` | file upload, object storage, URL generation |
| log | `log` | structured logging |
| search | `search` `search/meilisearch` | search abstraction and Meilisearch adapter |
| utils | `utils/idgenerator` `utils/secure` `utils/contextutil/*` `utils/encoding/*` | IDs, security helpers, metadata/context, encoding |
| infra | `infra/redis` `infra/sqlite` | infrastructure clients |

## 2) Group Details

### core

- Entry points:
  - `core/boot`: startup/shutdown/signal handling
  - `core/task`: managed long-running tasks
- Dependencies: usually wired at app entry (`cmd/app`).
- Pitfalls:
  - unmanaged long-running goroutines
  - panic paths without `core/safe`

### server

- Entry points:
  - `server/httpz`: response/error wrappers
  - `server/middleware/auth`: auth middleware
  - `server/auth/jwtauth`, `server/auth/acl`: token + access control
- Dependencies: commonly used with `httpx`.
- Pitfalls:
  - wrong middleware scope (public endpoints blocked or private endpoints exposed)
  - inconsistent business error mapping

### cache

- Entry points:
  - `cache/redis` for production
  - `cache/memory` for local/testing
  - `cache.GetEx` + singleflight for penetration control
- Dependencies: reachable Redis or memory fallback.
- Pitfalls:
  - unstable cache key design
  - missing TTL / negative cache strategy

### mq

- Entry points:
  - `mq/memory` for local/testing
  - `mq/redis` for production queue/pubsub
- Dependencies: serializable message model, idempotent consumers.
- Pitfalls:
  - queue semantics vs broadcast semantics mixed up
  - missing retry/idempotency behavior

### storage

- Entry points:
  - `storage/local`, `storage/s3`, `storage/qiniu`
  - `storage/urlhandler`, `storage/kvcache`
- Dependencies: valid credentials, bucket, and domain config.
- Pitfalls:
  - inconsistent object key strategy
  - storing URL only (without canonical key)

### log

- Entry points: `log.Init`, structured fields, `log.Sync`.
- Dependencies: single initialization at startup.
- Pitfalls:
  - repeated logger initialization
  - missing trace/request/user identifiers

### search

- Entry points: `search/meilisearch`.
- Dependencies: index schema aligned with data model.
- Pitfalls:
  - index not updated after data updates
  - query/sort fields not planned early

### utils

- Entry points:
  - `utils/idgenerator`
  - `utils/secure`
  - `utils/contextutil/metadata`
- Dependencies: deployment-compatible ID generator config.
- Pitfalls:
  - multiple ID strategies mixed in one service
  - default security settings used without review

### infra

- Entry points: `infra/redis`, `infra/sqlite`.
- Dependencies: validated timeout/retry/connection settings.
- Pitfalls:
  - client creation scattered in business code instead of centralized lifecycle management

## 3) Mapping to sphere-layout

| Need | Recommended Package(s) | Typical sphere-layout Location |
|---|---|---|
| app startup and composition | `core/boot` `core/task` | `cmd/app/*`, `internal/biz/*` |
| API implementation | `server/httpz` + generated code | `internal/service/api/*`, `internal/server/api/web.go` |
| auth and permissions | `server/auth/*`, `server/middleware/auth` | `internal/pkg/auth/*`, `internal/server/api/web.go` |
| caching | `cache/*` | `internal/pkg/*` or `internal/service/*` |
| queue/event flow | `mq/*` | `internal/biz/task/*` or `internal/service/*` |
| file storage | `storage/*` | `internal/service/shared/*` |
| infra/database setup | `infra/*` + Ent | `internal/pkg/database/*` |

## 4) Minimal Runnable Example Index

To avoid duplicated maintenance, reuse existing references:

1. API workflow examples: `references/02-api-development.md`
2. ORM/schema examples: `references/03-database-orm.md`
3. Auth/JWT/permission examples: `references/04-auth-permissions.md`
4. command/troubleshooting examples: `references/99-quick-reference.md`

## Default Selection Rules

1. Prefer Sphere abstractions before adding new base layers.
2. Use memory implementations for local/dev; switch to Redis/S3-like backends in production.
3. Stabilize API definitions and generation flow first, then optimize implementation details.
4. Add new capabilities in `internal/pkg` or `internal/service`, not new top-level directories.
