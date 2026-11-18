# Compatibility Baseline

`scripts/check-api-compat.sh` compares the current module with the latest
release tag using `golang.org/x/exp/cmd/apidiff`. The baseline tag is resolved
with `git describe --tags --abbrev=0 --match 'v[0-9]*'`; when that tag points
at HEAD the script falls back to the previous tag, since a tagged HEAD is the
release itself. Set `API_BASELINE_VERSION` to measure against a different tag.

`api-incompatibilities.txt` is the active allowlist: it records incompatible
changes, relative to the latest release tag, that are accepted. The check
fails when apidiff reports a change the file does not contain.

`api-incompatibilities-from-v0.0.3.txt` is the set apidiff reports against a
v0.0.3 baseline, the gate's previous baseline, kept for downstream upgraders
still coming from v0.0.3. It is a superset of the active allowlist rather than
a disjoint historical half, and the gate does not read it.

`behavior-changes.md` records changes that keep the same signature but change
runtime behaviour. `apidiff` cannot see those, so they are maintained by hand.

`internal/compatconsumer` is a source-level consumer fixture for the primary
public contracts and constructors. It is compiled by the normal test suite and
kept under `internal` so it does not add another public package to the module.
