# Repository Guidelines

## Project Structure & Module Organization

Sphere is a single Go module (`go.mod` at repo root) with domain packages grouped by concern. Core building blocks live
under `core` (boot wiring, typed errors, safe helpers, task orchestration) and are consumed by feature packages such as
`server` (HTTP/auth/reverse proxy), `cache`, `mq`, `search`, `social`, `storage`, `log`, and `utils`. Infrastructure and
deployment support files sit in `infra`, while generated assets or fixtures belong inside each package’s `test` or
`internal` subdirectory. Unit and integration tests are co-located with their packages and follow the `_test.go`
suffix (`server/httpz/error_test.go`, `cache/test/cache_test.go`), so explore the nearest folder when extending
functionality.

## Build, Test, and Development Commands

- `go test ./...` — quick way to confirm every package compiles and its tests pass; use before pushing any branch.
- `make lint` — runs `go fmt`, `go vet`, `go get`, `go test`, `go mod tidy`, `golangci-lint` (with gofmt/goimports
  rules), and `nilaway` for nil-safety; this is the authoritative “green” signal for CI parity.
- `go test ./cache/...` or similar scoped commands help iterate rapidly on a single package; mirror the folder path you
  touch.

## Coding Style & Naming Conventions

Code should be gofmt-clean (tabs for indentation, max line length left to gofmt). Keep packages lowercase and concise (
`search/meilisearch`), exported identifiers in UpperCamelCase, and unexported helpers in lowerCamelCase. Constructors
follow the `New<Type>` pattern, and context-aware functions accept `context.Context` as their first argument. Do not
hand-edit import order—`goimports` (via `make lint`) enforces canonical grouping. When touching interfaces, document
expectations with short comments placed immediately before the type to keep `golangci-lint` happy.

## Testing Guidelines

Prefer table-driven tests using `t.Run` for permutations, mirroring existing suites in `cache/test` and `utils/...`.
Name files `<feature>_test.go` and keep helper fixtures in the same directory or a `_testdata` folder. Aim to cover new
error paths and nil-handling since `nilaway` and the logger rely on predictable invariants. Run `go test ./path/...`
while developing, then `go test ./...` followed by `make lint` before opening a PR; CI expects both.

## Commit & Pull Request Guidelines

History shows `<type>: <imperative summary>` messages (`refactor: rename server/httpx...`). Follow that format (`feat`,
`fix`, `refactor`, `chore`, etc.) and keep summaries under ~70 characters. Each PR should include: a concise problem
statement, bullet list of changes, any screenshots/log snippets for behavioral updates, test evidence (`go test ./...`
output), and links to related issues or specs. Cross-reference directories you touched so reviewers can jump straight to
the relevant packages.
