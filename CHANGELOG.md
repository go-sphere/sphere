# Changelog

This file starts at v0.0.7. Earlier releases had no changelog; their
signature changes are listed in `compat/api-incompatibilities.txt` and their
runtime behaviour changes in `compat/behavior-changes.md`, both measured from
v0.0.3. The section "Earlier changes that shipped without a changelog" below
points at the ones most likely to surprise an upgrade.

## Unreleased (v0.0.7)

The release that removes process-global side effects from package init and
stops the storage layer from carrying HTTP semantics.

This release is **breaking**. Two of the four breaking changes compile cleanly
and only show up at runtime; each entry says what to write instead. The full
behaviour notes are under "The v0.0.7 breaking release" in
`compat/behavior-changes.md`.

### Breaking changes

**Importing `core/boot` no longer sets the timezone** (silent)

The package init used to run `InitTimezone(DefaultTimezone)`, forcing
`time.Local` and `TZ` to `Asia/Shanghai` in every binary that linked boot. It
no longer does; without an explicit call the process runs in the host's zone
(UTC in most container images), which shifts log timestamps, date formatting,
and cron/asynq schedules with an empty `Timezone`. `InitTimezone` and
`DefaultTimezone` are still exported. Call it first thing in `main`:

```go
if err := boot.InitTimezone(boot.DefaultTimezone); err != nil {
	// tzdata-less image: embed time/tzdata or accept the host zone.
}
```

**`idgenerator` no longer reads `WORKER_ID` in package init** (silent)

The global generator behind `idgenerator.NextId` is built once, on first use.
New: `idgenerator.Init(workerID uint16) error`, `idgenerator.InitFromEnv()
error` and `idgenerator.ErrAlreadyInitialized`. Without an explicit call the
first `NextId` reads `WORKER_ID` with the same rules as before (unset means 1,
valid `[0, 63]`); a malformed value now panics there — typically inside an ent
`DefaultFunc` on a request path — instead of aborting the process at startup.
`NextId` keeps its `func() int64` shape and IDs are unchanged for the same
worker ID. To keep failing at boot:

```go
if err := idgenerator.InitFromEnv(); err != nil {
	log.Fatalf("idgenerator: %v", err)
}
```

Initializing after the generator exists (including after a lazy `NextId`)
returns `ErrAlreadyInitialized`. The package no longer configures yitter's own
`idgen` global.

**`boot.WithLoggerInit` is removed**

It was the Deprecated zap-specific shortcut and the only reason `core/boot`
compiled in zap and lumberjack. Write instead:

```go
boot.WithLoggerBackend(zapx.NewBackend(conf, log.WithAttrs(map[string]any{"version": version})))
```

**Storage sentinels no longer carry an HTTP status; custom error parsers must
fall back to `httpz.ParseError`** (silent for custom parsers)

`storageerr.ErrNotFound`, `ErrDestExists` and `ErrFileNameInvalid` are plain
`errors.New` sentinels and no longer implement `httpx.StatusError`;
`storageerr` no longer imports `httpx`. The mapping (404 / 400 / 400, via
`errors.Is`) moved to the new `httpz.ParseError`, which is now the default
parser, so default rendering is unchanged. A parser registered with
`httpz.SetDefaultErrorParser` that falls back to `httpx.ParseError` now renders
these errors as **500**. Change the fallback:

```go
httpz.SetDefaultErrorParser(func(err error) (int32, int32, string) {
	// ... application-specific mappings ...
	return httpz.ParseError(err) // was: httpx.ParseError(err)
})
```

### Fixes

Storage driver fixes since v0.0.6, each described in
`compat/behavior-changes.md` under "Security and correctness fixes":

- `qiniu` server-side uploads overwrite existing keys, as the other drivers do,
  instead of failing with 614.
- `qiniu` reports a truncated download as an error wrapping
  `io.ErrUnexpectedEOF` instead of a short body and a clean `io.EOF`.
- `qiniu` `MoveFile` onto itself is a no-op (or `ErrNotFound` for a missing
  key) instead of `ErrDestExists`.
- `s3` downloads fetch body and metadata in one GET. The returned `Reader` no
  longer implements `io.Seeker`; callers that type-asserted it must buffer.

### Earlier changes that shipped without a changelog

These landed in v0.0.4 and were never announced outside `compat/`. They are
listed here because each changes what an upgrading caller sees:

- **`secure.CryptPassword` returns `(string, error)`.** The old single-value
  form fell back to plaintext on failure; every call site must handle the
  error. (`compat/api-incompatibilities.txt`)
- **Injected cache instances are not closed by `Close()`.**
  `redis.NewByteCache(client)`, `badgerdb.NewDatabaseWithBadger(db)` and
  `memory.NewMemoryCacheWithRistretto(cache, …)` no longer close what they were
  given, and the `CodecCache`/`NSCache` wrappers never close anything. Whoever
  constructed the backend closes it; the failure mode is a silent leak.
  (`compat/behavior-changes.md`, "`Close()` no longer closes what it did not
  create")
- **The HTTP error envelope changed.** `ErrorResponse.Error` is filled only
  with `httpz.SetDebugMode(true)`; `Message` is the status text unless the
  error carries an `httpx.MessageError`; `Code` is 0 for unclassified errors
  rather than repeating the status; a recovered panic answers with the fixed
  message "internal server error" instead of the panic value; and
  `AbortWithJsonError(ctx, nil)` writes a 500 instead of panicking.
  (`compat/behavior-changes.md`, "Responses no longer leak raw error text" and
  "`ErrorResponse.Code` is 0 for unclassified errors")
- **A task's `Start` error wrapping `context.Canceled` counts as a failure**
  while the run context is still live: `task.Group` stops the whole group and
  `boot.Run` returns it, and `task.Manager` reports it instead of discarding
  it. Only cancellation caused by the group's own teardown is ignored.
  (`compat/behavior-changes.md`, "`boot.Run` no longer swallows
  `context.Canceled` from `Start`" and "`task.Manager` no longer discards
  wrapped cancellation errors")
