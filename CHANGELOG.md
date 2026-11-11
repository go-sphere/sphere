# Changelog

This file starts at v0.0.7. Earlier releases had no changelog; their
signature changes are listed in `compat/api-incompatibilities.txt` and their
runtime behaviour changes in `compat/behavior-changes.md`, both measured from
v0.0.3. The section "Earlier changes that shipped without a changelog" below
points at the ones most likely to surprise an upgrade.

## Unreleased

### Added

- `stack.New` / `stack.Apply` (`server/middleware/stack`): assemble the
  standard engine-level middleware stack in one documented order, outermost
  first: telemetry Tracing, Metrics, RequestID, access Log, Recovery. Tracing
  and Metrics are opt-in (`WithTracing`, `WithMetrics`); RequestID and
  Recovery are on by default; the access log needs `WithLogger` and Recovery
  falls back to a stdio logger without one. The package documentation states
  the engine-level versus group-level rule (the stack and CORS on the engine,
  auth, rate limiting and selectors on groups). Existing middleware
  constructors are unchanged; layouts adopt it by calling `stack.Apply`.
- `ops.Handler`, `ops.NewServer`, `ops.HealthzHandler` (`server/ops`): a
  `/healthz` liveness endpoint and opt-in (`ops.WithPprof`) `net/http/pprof`
  on a dedicated `http.ServeMux`, served by `ops.Server`, a `task.Task` with
  its own listener. pprof is off by default and no function mounts it on an
  httpx engine; the package documentation spells out the exposure risk.
  Readiness probing is not provided. No new module requirements.
- `telemetry.NewTracing` and `telemetry.NewMetrics`
  (`server/middleware/telemetry`): OpenTelemetry server span and
  `http.server.request.duration` / `http.server.active_requests` middleware on
  the OpenTelemetry API only (no SDK). Spans and metrics are labelled with the
  matched route (`unmatched` in metrics, no `http.route` on unmatched spans),
  5xx marks the span Error and 4xx does not, an unrecovered panic still ends
  the span and records the duration, and `telemetry.TraceAttrs` supplies
  `trace_id`/`span_id` for logs. `go.opentelemetry.io/otel`, `otel/metric` and
  `otel/trace` become direct requirements of `sphere` at the versions it
  already resolved; `go.sum` is unchanged.
- `log.ContextWithAttrs` and `log.AttrsFromContext`: attach attrs to a
  `context.Context`; loggers built by the `log` package append them to every
  `*Context` entry.
- `requestid.New` (`server/middleware/requestid`): request-ID middleware that
  publishes the ID to the context, the httpx state store and the response
  header, and as a `request_id` log attr. Client-supplied IDs are accepted only
  when at most 64 characters of `[A-Za-z0-9._-]`; `WithAlwaysGenerate` never
  trusts the client.
- `logger.UnmatchedRoute`, the `route` value for requests that matched no route.
- `httpz.HandlePanic`, the recover tail shared by `httpz.WithRecover` and
  `logger.RecoveryLog`; `logger.RecoveryLogErr` and `logger.PanicError`, an
  opt-in form of `RecoveryLog` that returns the recovered panic as an error.

### Changed

- `logger.Log` access entries gain a `route` attr (the matched pattern, or
  `unmatched`). `logger.Log` and `logger.RecoveryLog` write through
  `InfoContext`/`ErrorContext` with the request context when the logger
  implements `log.ContextLogger`, so ctx attrs such as `request_id` appear;
  loggers without it are called as before. Contract tests:
  `TestLogRouteAttr`, `TestLogUsesContextLogger`.
- `logger.RecoveryLog` no longer writes a 500 when the response is already
  committed, matching `httpz.WithRecover`. `httpz.AbortWithJsonError` calls
  `ctx.Committed()` directly instead of asserting an optional interface.
- `httpz.ParseError` maps `context.DeadlineExceeded` to 504 instead of 500.
  An `httpx.StatusError` in the chain still takes precedence, and
  `context.Canceled` is unchanged. Contract test:
  `TestParseError_DeadlineExceededIs504`.
- `reverseproxy`: the default cache key is the inbound Host followed by the
  request URI. The Host is client-chosen and reaches the upstream as
  `X-Forwarded-Host`, so a URI-only key let one forged-Host request poison the
  entry every client is served. Entries stored under the old key are no longer
  found; their bodies stay in storage until removed. A later `WithTargetURL`
  now wins over an earlier one for the default director.
- `cache/memory`: `NewMemoryCache`, `NewByteCache` and `NewMemoryCacheWithCost`
  size ristretto's admission counters for about 100k entries (1e6 counters)
  instead of 1e7, so an empty cache costs about 4 MB of heap instead of 48 MB.
  This also applies to the default caches of `ratelimiter` and the `file`
  service. Use `NewMemoryCacheWithRistretto` for caches far larger than that.
- `log`: an attr passed at the call site wins over a `ContextWithAttrs` attr
  with the same key, and each key appears once per entry; with
  `WrapBackendWithContextMerge` the context value used to win, and without it
  both were written.
- `boot.Run`: when a before-start hook fails, the after-stop hooks still run
  (so `WithLoggerBackend` syncs and builder-owned clients close), and a
  builder returning a nil `*Application` without an error is a build failure
  instead of a panic.
- `httpz.WithSSE`: a request whose context ends before the first frame is
  rendered by the error parser, so a deadline answers 504 instead of a fixed
  500. A client disconnecting mid-stream is no longer logged at Error level,
  and heartbeat comments are sent only on idle streams.
- `httpz.Fs` returns an error when `emPath` does not name a directory in the
  embedded filesystem, instead of an empty FS that answers 404 for everything.
- `telemetry.NewMetrics` records no `error.type` for a request failed by its
  own client's cancellation, matching `NewTracing`.
- `scheduler/asynq`: once a drain that outlived every `Stop` deadline
  finishes, the scheduler is closed, as with cron: `Register` and `Enqueue`
  return `ErrClosed`, and so does a later `Close`.
- `task.Group.Start` rejects a typed-nil member or one whose `Identifier`
  panics before starting anything, instead of panicking out of `Start` and
  leaving the group unable to stop.
- `cache.WithSingleflight`: a caller whose context ends while waiting for the
  shared build returns `ctx.Err()`; the build and its cache write still
  complete for the other callers. A builder panic reaches every waiter with the
  builder's own panic value.
- `mq/memory`: `Publish` and `Consume` with an already-done context return
  `ctx.Err()` without enqueuing or taking a message, as the redis driver does.
- `s3`, `kvcache`: `CopyFile`/`MoveFile` from a missing source onto an existing
  destination fail with `storageerr.ErrNotFound`, like local and qiniu, instead
  of `ErrDestExists`. `s3` `CopyFile` of a key onto itself no longer sends a
  `CopyObject` that S3 rejects: it succeeds with overwrite and fails with
  `ErrDestExists` without.

### Fixed

- `httpz.WithJson`/`WithText` no longer append the success envelope to a
  response the handler already committed itself (a redirect or a file).
- `cors` appends `Origin` to `Vary` instead of replacing headers an outer
  middleware set there, such as compression's `Accept-Encoding`.
- `cache.CodecCache.GetDel` returns the zero value and an error wrapping
  `ErrDecode` for an undecodable entry, like `Get`.
- `badgerdb`: `MultiSet`, `MultiSetWithTTL` and `MultiDel` split a batch that
  exceeds badger's transaction size across several transactions instead of
  failing with `ErrTxnTooBig`.
- `numconv.Base62ToInt64`/`Base32ToInt64` reject oversized input before
  decoding it; a long string from a URL cost CPU quadratic in its length.
  `baseconv` encodes and decodes non-power-of-two bases without allocating on
  every digit.
- `log`: StdioBackend prints errors wrapped dozens of times with `%w` instead
  of the unformattable placeholder, walks shared subtrees once, and bounds the
  work for a `LogValuer` group that refers to itself more than once.
  Backend preset attrs are rendered once, not on every entry.
- `search/meilisearch`: concurrent `Index` calls on one `Searcher` no longer
  race, and `Index`/`Delete` poll task status at the client's default interval
  instead of waiting at least a second.
- `scheduler/cron`: `WithLogger(nil)` keeps the default logger instead of
  crashing the process on the first overlapping run.
- `local`, `s3`: `ListFiles` with a huge `limit` no longer panics or
  preallocates for it, and `s3` requests only one key past the page.
- `docs`: the relaxed CORS wrapper advertises `PATCH` instead of the
  nonexistent `UPDATE`.
- A nil `Option` is ignored by `boot`, `pool`, `auth`, `ratelimiter`,
  `requestid`, `stack`, `telemetry` and `reverseproxy`, as it already was
  elsewhere.
- Logs written by `httpz.AbortWithJsonError`, `MatchOperation` and the
  `WithSSE` stream carry the request context, so `request_id`/`trace_id`
  appear on them.
- `log`: package-level `DebugContext`/`InfoContext`/`WarnContext`/
  `ErrorContext` include attrs attached with `ContextWithAttrs`. `logbuffer`
  stores error attrs, including preset ones, as their message, so entries
  serialize to JSON as text instead of `{}`; a typed nil error is recorded as
  nil and a panicking attr no longer drops the attrs after it.
- `httpz`: `WithRecover` and `WithSSE` panic logs carry the request context, so
  `request_id`/`trace_id` appear on them. A `send` made after a `WithSSE`
  stream function returned no longer crashes the process with "send on closed
  channel"; it blocks until the stream is torn down and returns the
  cancellation cause, also when the send is made while the pump is still
  writing, and a failure before the pump runs (retry frame, initial flush)
  cancels the producer, including on adapters that run the stream callback
  after the handler returned and drop its error (fiber).
- `logger.Log` records the status the client receives when the chain returns an
  error: the recorded status once the response is committed, otherwise the
  status `httpz.ErrorStatus` resolves.
- `ratelimiter` no longer builds and leaks a default in-memory cache, with its
  background goroutines, when `WithCache` is supplied.
- `jwtauth.ParseToken` decodes JSON numbers exactly when the claims type is a
  pointer such as `*RBACClaims[int64]`, so `int64`/`uint64` IDs above 2^53 are
  no longer rounded through `float64`.
- `docs` proxy keeps percent-encoded path segments such as `%2F` when
  forwarding, and `doc.json` keeps integers beyond 2^53 verbatim.
- `reverseproxy`: `WithSaveTimeout` is an idle timeout that restarts whenever
  body bytes reach the cache, so slow downloads are cached instead of aborted
  at a fixed deadline; a stalled upstream or backend is still abandoned after
  the timeout. The timeout also closes the cache body queue, so a save parked
  in a driver that ignores its context (local storage) is released.
- `jwtauth.ParseToken` rejects a payload with bytes after the JSON object for
  pointer claims too, as it already did for value claims.
- `scheduler/asynq`: `Stop` returns the error of a panicking teardown step
  (the remaining steps still run) instead of reporting a clean drain, and stops
  the server's processor before waiting for cron to shut down.
- `qiniu`: SDK calls (region query, uploads, management and download requests)
  use https instead of the SDK's plaintext http default; users whose hosts
  were reachable only over http must set an explicit `http://` UC host. The
  upload-token TTL doc says it is truncated to whole seconds.
- `local`: `ListFiles` works when `RootDir` is a symlink, and keys whose parent
  path component is a regular file are reported as missing (`IsFileExists`
  false, `ErrNotFound`, idempotent `DeleteFile`) instead of failing with a raw
  "not a directory" error.
- `s3`: `ListFiles` returns the caller's context error when the context ends
  before the listing is exhausted, instead of a truncated page with an empty
  cursor that reads as end of list.
- `cache/memory`: `NewMemoryCacheWithCost(nil)` costs every item as 1 like
  `NewMemoryCache`, and `NewByteCache` costs empty values as 1, so zero-cost
  entries count against `MaxCost` and are evicted.
- `scheduler/asynq`: `Stop` no longer holds the scheduler lock while cron and
  the asynq server drain, so it returns `ctx.Err()` at the caller's deadline
  when Redis stalls and concurrent `Register`, `Unregister` and dispatch are
  not blocked during the drain.

## v0.0.7 (2026-10-08)

The release that removes process-global side effects from package init and
stops the storage layer from carrying HTTP semantics.

This release is **breaking**. Three of the four breaking changes compile
cleanly and only show up at runtime (the storage one only for custom error
parsers); each entry says what to write instead. The full
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

Initializing again (including after a lazy `NextId` or a failed attempt)
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

**`jwtauth.ParseToken` rejects tokens without `exp`** (silent)

A token whose claims carry no `exp` used to validate forever; it now fails
with `jwt.ErrTokenRequiredClaimMissing`. Tokens built with `NewRBACClaims`
always carry `exp` and are unaffected. Set `ExpiresAt` on custom claims.

**`fileserver` namespaces upload tokens; `WithCreateFileKey` only generates
the token**

Upload tokens are stored under the `sphere-upload-token:` prefix in the
injected cache, so the public PUT route can no longer read or delete other
entries of a shared cache by naming their key in the URL. Tokens issued before
the upgrade stop working. `WithCreateFileKey` now takes
`func(ctx context.Context) (string, error)` and returns just the token; the
FileServer stores it with the resolved TTL.

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
- `s3` `UploadFile` buffers 16 MiB per upload instead of ~537 MiB. The new
  `Config.PartSize` (default 16 MiB, minimum 5 MiB) sets the buffer and caps
  objects uploaded through `UploadFile` at `PartSize * 10000` (~156 GiB by
  default); `UploadLocalFile` is unaffected.

Other fixes:

- `task.Manager` no longer deadlocks when a running task calls `StartTask`
  while `Wait` or `StopAll` is in progress; `Wait` also waits for tasks
  started that way.
- `urlhandler` (and the s3, qiniu and fileserver URLs built on it) escapes each
  key segment, so keys containing `%`, `?`, `#` or spaces produce URLs that map
  back to the same key instead of a different one or `""`.
- `httpz.WithRecover` (and so `WithJson` and the other wrappers) and
  `httpz.AbortWithJsonError` no longer append a JSON error document to a
  response the handler already committed; the error or panic is logged at
  Warn level and nothing more is written. This needs a context exposing
  `Committed() bool`, which `httpx.Context` gains after v0.0.5; under httpx
  v0.0.5 the error is still written.
- The read-through loaders `cache.GetEx`, `GetObjectEx` and `GetJsonEx` treat
  an undecodable cached entry (for example one written under an older schema)
  as a miss: the builder runs and overwrites it, instead of every read failing
  until the entry expires. New `cache.ErrDecode`: `GetObject`, `GetJson` and
  `CodecCache.Get` return decode failures wrapping it (the codec error is
  still reachable with `errors.As`), together with the zero value instead of
  a partially decoded one. With a nil builder the loaders return the decode
  error, since nothing can rebuild the entry.
- `memory` caches set ristretto's `IgnoreInternalCost`, so `UpdateMaxCost(n)`
  holds n items (`NewMemoryCache`) or n value bytes (`NewByteCache`) instead of
  far fewer once ristretto's per-item overhead was added to each cost.
- `nscache.NSCache.DelAll` deletes in batches of at most 1000 keys, so a large
  namespace over badger no longer fails with `ErrTxnTooBig`.
- `scheduler/cron`: when the last waiting `Stop` times out, the handler
  context is cancelled, so a job watching `ctx.Done()` can abort before the
  process exits instead of running on until it is killed.
- `jwtauth.ParseToken` errors are 401 `httpx.StatusError`s wrapping the
  golang-jwt error (`errors.Is(err, jwt.ErrTokenExpired)` still matches), so a
  handler returning them unchanged, such as a refresh endpoint given an
  expired token, renders 401 instead of 500.
- New `fileserver.WithMaxUploadSize(n)` bounds the `RegisterFileUploader` PUT
  body: a declared `Content-Length` over n answers 413 before the token is
  spent, and a body streaming past n is cut off with 413. Uploads stay
  unbounded by default.
- `httpz.WithFormFileReader`/`WithFormFileBytes` answer an oversize file with
  **413 instead of 400** (behaviour change for clients matching on the status),
  and reject a declared `Content-Length` above `WithFormMaxSize` plus 1 MiB
  before the multipart body is parsed. Chunked bodies are still parsed in full
  by the adapter before the size check.

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
