# Repository Guidelines

This repository is a Go library that provides building blocks for Sphere-based services. Keep changes small, tested, and consistent with the existing structure.

## Project Structure & Module Organization
- `core/`: boot, tasks, error and safety primitives.
- `server/`: HTTP helpers (Gin), auth, middleware, service utilities.
- `database/`: bindings, mappers, SQLite helpers.
- `cache/`: cache interfaces and implementations (memory, redis, badger) + tests.
- `storage/`: object storage backends and URL handlers.
- `infra/`: infrastructure clients (e.g., redis).
- `mq/`, `search/`, `social/`, `utils/`, `log/`: message queue, search, integrations, utilities, logging.
- Tests live alongside packages as `*_test.go`.

## Build, Test, and Development Commands
- `make lint`: format, vet, tidy, run `golangci-lint` and `nilaway`; also runs `go test ./...`.
- `go test ./...`: run the test suite.
- `go test -race -cover ./...`: race detector with coverage.
- `go build ./...`: compile all packages (library only; no binary).

## Coding Style & Naming Conventions
- Use Go 1.24+. Always `gofmt`/`goimports` (enforced by `make lint`).
- Packages: lowercase, short, no underscores (e.g., `server/ginx`).
- Exports: PascalCase for types/functions, non-exports: lowerCamel.
- Errors: use `errors` package patterns; prefer wrapped errors and typed errors where present.
- File names mirror the concept (e.g., `logger.go`, `storage.go`).

## Testing Guidelines
- Place tests in the same package: files end with `_test.go` and functions `TestXxx(t *testing.T)`.
- Prefer table-driven tests and subtests.
- Aim for meaningful coverage on new/changed logic; run `go test -race -cover ./...` before pushing.

## Commit & Pull Request Guidelines
- Commits: Conventional style (e.g., `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`) as seen in history.
- PRs: clear description, motivation, and scope; link issues; note breaking changes; include tests and examples.
- CI must pass. Run `make lint` locally before opening a PR.

## Security & Configuration Tips
- Do not commit secrets or access tokens; use environment variables or local config.
- Prefer constructor functions that accept dependencies (for testing and injection).
- When adding integrations (e.g., storage, mq, redis), provide minimal, secure defaults and clear configuration comments.

