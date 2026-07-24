# Compatibility Baseline

`scripts/check-api-compat.sh` compares the current module with `v0.0.3` using
`golang.org/x/exp/cmd/apidiff`.

`api-incompatibilities.txt` records incompatibilities that already existed
after `v0.0.3`. The check accepts those historical entries but fails when a new
incompatible change appears.

`internal/compatconsumer` is a source-level consumer fixture for the primary
public contracts and constructors. It is compiled by the normal test suite and
kept under `internal` so it does not add another public package to the module.
