# Contributing to Sphere

Thank you for your interest in contributing to Sphere! Please feel free to open an issue or submit a pull request.

## Development Workflow

Run formatting, static analysis, tests, race detection, and public API compatibility checks before submitting changes:

- `make fmt` — format Go files and imports
- `make check` — verify `go mod tidy -diff`, `lint`, and unit tests
- `make verify` — run `make check` plus `go test -race ./...`
- `make api-compat` — verify exported Go API compatibility against the baseline release

Preserve backward compatibility of exported Go types, functions, methods, interfaces, and constants unless a breaking change has been explicitly discussed and approved.
