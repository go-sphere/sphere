# Behaviour Changes since v0.0.3

`api-incompatibilities.txt` is produced by `apidiff` and only sees **signature**
changes. This file records changes where the signature is identical but the
runtime behaviour is not — exactly the changes a compiler cannot catch and an
upgrade can silently break.

Scope: the per-package pass below covers `cache/` only. The other top-level
packages have not had a full behavioural review, but the security and
correctness fixes listed under "Security and correctness fixes" and the
confirmed silent changes under "Silent changes outside `cache/`" change
observable behaviour outside `cache/` and are recorded there.

## Read this first — silent changes

These compile cleanly, raise no error at runtime, and change what ends up
stored. They are the ones worth auditing call sites for.

### Entries that used to expire may now live forever

The TTL contract was unified across drivers (`cache/cache.go`): `expiration > 0`
expires, `expiration == 0` never expires **and clears any existing TTL**, and
`expiration < 0` returns the new `cache.ErrInvalidTTL` without writing.

Three drivers changed as a result, and all three fail in the same direction —
keys that previously disappeared now persist:

| Driver | `Set` / `SetWithTTL(…, 0)` before | after |
| --- | --- | --- |
| `redis` | `Set` issued `SET … KEEPTTL`, preserving the existing TTL | `SET` without `EX`/`PX`, clearing the TTL |
| `mcache` | `0` recorded an expiry of *now*, so the entry was immediately gone | never expires |
| `badgerdb` | `WithTTL(0)` set `ExpiresAt = now`, so the write was unreadable straight away | never expires |

Audit any call site that writes with a computed or zero TTL. Under `redis` a
`Set` used to top up a value while keeping its original expiry; it now makes the
key permanent. Under `mcache` and `badgerdb` a computed `0` used to mean "do not
keep this"; it now means "keep this forever".

### `-1` no longer means "never expire" on mcache

`mcache` previously treated any negative expiration as "never expire" and `0` as
"already expired" — the exact inverse of the new contract. `SetWithTTL(…, -1)`
now returns `cache.ErrInvalidTTL` and stores nothing. Note that
`cache.WithNeverExpire()` and the `newOptions` default both use `-1`
internally, so code written against this package may well have copied the same
convention. Use `0` for entries that never expire.

### `Close()` no longer closes what it did not create

Ownership is now explicit: a constructor that **creates** a resource closes it;
a constructor that **receives** one does not. This applies at both layers.

Drivers — `Close()` is a no-op when the resource was injected:

- `redis.NewByteCache(client)` — use `redis.NewByteCacheWithOptions(opts)` to
  get a cache that owns and closes its client.
- `badgerdb.NewDatabaseWithBadger(db)` — `NewDatabase` and
  `NewDatabaseWithOptions` still own and close theirs.
- `memory.NewMemoryCacheWithRistretto(cache, …)` — `NewMemoryCache` and
  `NewMemoryCacheWithCost` still own and close theirs.

Wrappers — `Close()` is always a no-op, because they can only ever wrap an
injected cache:

- `cache.CodecCache` (`NewCodecCache`, `NewJsonCache`)
- `nscache.NSCache` (`NewNSCache`)

Consequence: there is no longer any way to close a backend *through* a wrapper.
Whoever constructed the backend must close it. The failure mode if you miss
this is a leak, not an error — and on `memory` the opposite mistake was silent
too: a propagated `Close` turned every sibling cache into a permanent miss with
no error, because ristretto reports reads on a closed cache as not-found.

## Silent changes outside `cache/`

The scope note above was written before these were confirmed. All six are
behaviour changes with identical signatures, verified against the source and
recorded here because nothing else in this file covers them.

### Redis connectivity is no longer verified at construction

`infra/redis.NewClient` used to parse the URL, create the client, and run a
`Ping` — an unreachable server or bad credentials failed at construction, which
for most deployments means failed at boot. It now only parses the URL and
builds the client; go-redis connects lazily on first use, so the error surfaces
at the first command instead. For a background consumer that can be minutes
later and surface only as a log line; on a request path it is a 500 that
startup never noticed. Callers that used the constructor as a connectivity
check must `Ping` explicitly — the return is a plain `*redis.Client`, so
`client.Ping(ctx).Err()` is available. (`infra/redis/client.go:16`)

### Meilisearch `Index`/`Delete` no longer ignore a failed task

`Searcher.Index`/`Delete` waited for the indexing task and discarded the
result. `WaitForTaskWithContext` returns `(task, nil)` even for `failed`/
`canceled` terminal states, so a rejected task was reported as success — the
document silently absent from the index, a `Delete` Meilisearch refused
indistinguishable from a real one. Both now pass the result through
`taskError`, which returns `meilisearch: task N <status>: <message> (<code>)`
for anything but `TaskStatusSucceeded`. Writes that previously returned nil
when the task failed now fail loudly, with a message that names the status and
the server's error code, and a caller that reasoned "nil means accepted" has
to distinguish "accepted and indexed" from "accepted then failed".
(`search/meilisearch/meili.go:73`, `:85`)

### `boot.Run` no longer swallows `context.Canceled` from `Start`

`Application.Start` used to return nil for any error satisfying
`errors.Is(err, context.Canceled)`. It now defers to the group: only
cancellation the group's own teardown provoked is discarded, and a `Canceled`
result that arrives while the run context is still live counts as a failure —
even when wrapped. A clean signal shutdown still returns nil, because the group
already filters the cancellation its own teardown causes. What changed is the
race: a task reporting a wrapped `context.Canceled` at about the same time the
signal arrives used to be dropped and the run exited 0; it now surfaces in
`Run`'s result, and a program that maps a non-nil result to a non-zero exit
code fails a shutdown that used to look clean. (`core/boot/app.go:37`,
`core/boot/run.go:54-58`; the group's own filter is `core/task/group.go:377`)

### `ErrorResponse.Code` is 0 for unclassified errors

`AbortWithJsonError` used to take the parser's code verbatim. `httpx.ParseError`
falls back to `code = status` when the error does not implement
`httpx.CodeError`, so an unclassified 500 was also reported with an application
code of 500 — conflating the transport status with the value `Code` exists to
carry. The code is now normalized: when the error does not implement
`httpx.CodeError`, `Code` is always 0 and only the HTTP status carries the
transport semantics. This applies to a custom `SetDefaultErrorParser` too — a
non-zero code for an unclassified error is normalized away regardless of which
parser produced it. Clients that matched on status-like codes now see 0.
(`server/httpz/error.go:76-79`)

### Base32 identifiers changed alphabet — old values no longer decode

`AlphabetBase32` is a different alphabet. v0.0.3's was 33 symbols — `L` present,
`U` absent, contradicting its own comment that excluded both — and v0.0.4's is
32-symbol Crockford base32 (I, L, O, U excluded, which is what the comment
always claimed). This is a data-format change, not a constant rename:

- A v0.0.3 string containing `L` fails to decode outright — `DecodeString`
  rejects any character outside the current alphabet with
  `invalid character 'L'` (`utils/encoding/baseconv/baseconv.go:197-201`).
- Every other v0.0.3 string decodes to **different bytes**: removing `L` shifts
  the per-character value of `M`–`Z` by one, and the base itself changed 33→32,
  so the byte packing differs for every multi-character string. (Single
  characters coincide, by their digit value being unchanged.)

Any identifier persisted under v0.0.3 — an ID, a code, a cache key — is not
readable under v0.0.4: it stops decoding or silently maps to different data.
Re-encode, or keep the v0.0.3 alphabet around as a migration table.
(`utils/encoding/baseconv/basenum.go:6`; the constant change is also in
`api-incompatibilities.txt`)

### `afterStop` hooks no longer share an expired shutdown context

When `Stop` consumes the whole `WithShutdownTimeout` budget, `afterStop` used
to observe `context.DeadlineExceeded` on the same context. Hooks that honoured
the context — a final flush — failed and could turn a late shutdown into a
non-zero exit. They now get a short fresh context (2s) in that case. A hook
that must still run longer needs its own timeout. (`core/boot/run.go`)

### `Group.Stop(ctx)` bounds member `Stop`, not only the caller's wait

The ctx passed to `Group.Stop` used to bound only `waitForDone`. Member
`Task.Stop` always received a separate `WithCleanupTimeout` context from
`Background` (default 30s). `boot.WithShutdownTimeout(5s)` therefore returned
in 5s while members still believed they had 30s, and `Run` could return while
`Group.Start` was still in `stopTasks`.

A `Stop(ctx)` that requested shutdown now hands members
`min(ctx deadline, cleanup timeout)`. Internal teardown (task failure, parent
cancel, natural complete) still uses cleanup timeout alone. `boot.Run` calls
`Stop` before cancelling the run context so that deadline is the one members
see. Tasks that ignore their `Stop` ctx still run as long as they like;
`waitForDone` still returns when the caller ctx expires.

Callers that relied on "Run already returned, handlers still draining on the
cleanup clock" must set a longer `WithShutdownTimeout` (or 0 for unbounded).
(`core/task/group.go`, `core/boot/run.go`)

### `Stop` before `Start` no longer launches members

A `Group.Stop` that arrived before `Start` used to start the first stage and
immediately stop it, so a one-shot's `Start`/`Stop` both ran. Stages whose
start never began now receive neither call, matching the staged-group contract
already documented for later waves. A parent context that is already cancelled
when `Start` is invoked likewise starts nobody. (`core/task/group_run.go`)

### Scheduler `Start` no longer tears the runtime down

`scheduler/cron` and `scheduler/asynq` `Start` used to call `Stop` with
`context.WithoutCancel` after the run context ended, so a Group's cleanup
timeout could not bound drain: `Start` waited on an unbounded `Stop`. They now
return when the run context is done. `Stop` is the cleanup half of `task.Task`;
cancelling the context without `Stop` leaves cron/asynq live. `boot.Run` and
`task.Group` always `Stop`. (`scheduler/cron/scheduler.go`,
`scheduler/asynq/scheduler.go`)

## Per package

### `cache` (contract, wrappers, loader)

- New: `cache.ErrInvalidTTL`, `cache.ErrNotSupported`,
  `cache.ErrTTLCalculatorType`, and the optional `cache.KeyLister` interface.
- `WithDynamicTTL` + `SetObject`/`SetJson`/`GetObjectEx`/`GetJsonEx`: the
  calculator now receives the **original value** `T` rather than the encoded
  bytes. This is the fix that makes `WithDynamicTTL[T]` usable — previously it
  panicked on that path and only `WithDynamicTTL[[]byte]` worked. A calculator
  instantiated for the wrong type now fails the write with
  `ErrTTLCalculatorType` instead of panicking.
- `WithDynamicTTL` given a nil calculator: the write now fails with
  `ErrTTLCalculatorType` instead of panicking on the nil call. The same
  caveat as the type mismatch applies on the `GetEx` family: the setter
  error is swallowed there, so a nil calculator yields a successful load
  that just skips the cache write.
- `CodecCache.Keys` forwards prefix listing to the underlying byte cache, or
  returns `ErrNotSupported` when it has no `KeyLister`.
- `CodecCache.Close` is a no-op — see above.

### `cache/redis`

- `Set`/`MultiSet` clear the TTL instead of preserving it — see above. There is
  no longer any way to express `KEEPTTL` through this API; hold your own
  `*redis.Client` if you need it.
- `SetWithTTL`/`MultiSetWithTTL` reject a negative expiration with
  `ErrInvalidTTL`. `redis.KeepTTL` is `-1`, so explicit uses now fail.
- `DelAll` issues `FLUSHDB` instead of `FLUSHALL`: it no longer wipes every
  database on the instance. It still clears the whole selected database, which
  is wider than the cache if that database holds anything else.
- `MultiGet`/`MultiDel` with an empty key slice return an empty result / do
  nothing, instead of failing with a wrong-number-of-arguments protocol error.
- `Close` does not close an injected client — see above.
- New: `Keys(prefix)`, backed by `SCAN` with glob metacharacters escaped.

### `cache/mcache`

- TTL semantics for `0` and negative values are inverted relative to v0.0.3 —
  see above.
- `Get`/`MultiGet` take a write lock. They delete expired entries in place,
  which under the previous read lock was a `map` write racing with concurrent
  readers. Reads no longer run in parallel.
- New: `Keys(prefix)`. Note it returns `[]string`; for a `Map` with a non-string
  key type the keys are rendered with `fmt.Sprint` and cannot be fed back into
  `MultiDel`.

### `cache/badgerdb`

- `SetWithTTL`/`MultiSetWithTTL` with `0` store a permanent entry instead of one
  that is immediately expired — see above. Negative expirations return
  `ErrInvalidTTL`.
- `GetDel` retries internally on `badger.ErrConflict`. BadgerDB runs `Update`
  transactions optimistically and `GetDel` is the only method whose read set
  overlaps its write set, so concurrent callers on one key previously saw a
  retryable conflict surface as a plain error.
- `Close` does not close an injected `*badger.DB` — see above.
- New: `Keys(prefix)`.

### `cache/memory`

- `Set`/`MultiSet` no longer return an error when ristretto drops the write.
  A dropped write is normal under load — ristretto's `true` return does not
  guarantee retention either — so it is reported as success rather than as a
  non-deterministic error. Callers that branched on that error now see `nil`.
- `MultiSet`/`MultiSetWithTTL` respect `allowAsyncWrites`. They previously
  always waited for the write buffer to drain regardless of the setting, so
  with async writes enabled a read immediately after `MultiSet` may now miss.
- `SetWithTTL`/`MultiSetWithTTL` reject a negative expiration.
- `GetDel` is atomic: an entry is reported as found to at most one caller.
  It previously read and deleted without a lock, so concurrent callers could
  each consume the same one-shot entry.
- `SetAllowAsyncWrites` is safe to call concurrently with `Set`/`MultiSet`.
- `Close` does not close an injected ristretto cache — see above.
- This driver does not implement `KeyLister`, which is what makes
  `NSCache.DelAll` return `ErrNotSupported` over a memory backend.

### `cache/nocache`

- `SetWithTTL`/`MultiSetWithTTL` reject a negative expiration with
  `ErrInvalidTTL` instead of accepting it. Turning caching off should change
  what is persisted, not which arguments are legal, so a latent negative TTL is
  no longer masked by switching to this driver.

### `cache/nscache`

- `DelAll` deletes only the keys in its own namespace instead of delegating to
  the backend's `DelAll`. Sharing one backend between namespaces is now safe.
  It requires the wrapped cache to implement `KeyLister`, so over a `memory` or
  `nocache` backend it returns `ErrNotSupported` where it previously wiped the
  backend. It is also not atomic: keys written between the scan and the delete
  survive.
- New: `Keys(prefix)`, with the namespace prefix stripped from the result so an
  `NSCache` can be nested inside another one.
- Namespaces must be flat and must not contain `":"`. Isolation is by key
  prefix, so with namespaces `a` and `a:b` the keys of `a:b` also carry the
  `a:` prefix and `DelAll`/`Keys` on `a` would reach into `a:b`.
- `Close` is a no-op — see above.

## Security and correctness fixes

Each of these closes a defect that was reachable from ordinary usage. Several
tighten what is accepted, so a deployment that relied on the old, broken
behaviour will now see an error where it previously saw silent success.

### Responses no longer leak raw error text

`httpz.AbortWithJsonError` used to place `err.Error()` in `ErrorResponse.Message`
for any error that does not implement `httpx.MessageError` — the debug-mode
guard only ever covered the `Error` field. Driver and database strings
(credentials, hostnames, SQL) were therefore returned to clients in production.
`Message` now falls back to the generic HTTP status text unless the error
carries an explicit user-facing message, which also replaces the empty `Message`
that classified-but-message-less errors used to produce.

### A token without a `uid` claim is rejected

`jwtauth.RBACClaims.GetUID` returned `(zero, nil)`. Because the claim is
serialized with `omitempty`, any token signed with the same key for another
purpose (password reset, email verification, an internal service token) parsed
into UID 0 and authenticated as that user. It now returns
`authorizer.MissingUIDError`, which the auth middleware turns into a 401.
Callers that legitimately use 0 or `""` as a user ID must switch to a non-zero
identifier.

### CORS rejects `"*"` combined with credentials

`cors.NewCORS` now returns `(httpx.Middleware, error)` and fails with
`cors.ErrWildcardWithCredentials` for that combination. Previously it reflected
the caller's own `Origin` back with `Access-Control-Allow-Credentials: true`,
which let any site read authenticated responses using the victim's cookies.
Per-origin wildcards such as `https://*.example.com` are unaffected.

### The reverse-proxy cache no longer stores private responses

The default response check rejects a response when the request carried
`Authorization` or `Cookie`, when the response sets a cookie or declares
`no-store`/`private`, or when it carries a `Vary` the URI-only cache key cannot
satisfy. `Set-Cookie` is also stripped before persisting, covering custom
checkers. Previously a credentialed response — body and session cookie — was
cached under the request URI and replayed to anonymous callers. Deployments that
intentionally cached per-user responses must supply their own
`WithCacheKeyFunc`/`WithResponseCacheCheck`.

### File downloads are served as attachments

`fileserver`'s download endpoint now always sends
`X-Content-Type-Options: nosniff` and, by default,
`Content-Disposition: attachment`. The content type is derived from the key's
extension, so an uploaded `.html` or `.svg` previously executed on the origin
serving `GetBase` — which the bundled file service points at the same base URL
as uploads. Use `WithInlineDownload()` to restore inline rendering when
`GetBase` is a session-free origin.

### `WORKER_ID` accepts 0 and rejects garbage

`idgenerator` treated `WORKER_ID=0` as invalid and fell back to 1, so under the
standard StatefulSet pattern of deriving the value from a pod ordinal, pod-0 and
pod-1 shared a worker ID and emitted colliding snowflake IDs. Zero is now valid
(the generator's range is `[0, 63]`), and a malformed or out-of-range value
panics during package init instead of silently falling back — guessing an ID
cannot preserve the uniqueness this package exists to provide.

### Captcha rejects empty codes and normalizes its config

`NewManager` now defaults non-positive `CodeLength`/`CodeExpiresIn` to
`DefaultCodeLength`/`DefaultCodeExpiresIn`. A zero `CodeLength` used to generate
an empty code that `Verify` then accepted from anyone; a zero `CodeExpiresIn`
expired every code as it was stored. `SaveCode` refuses an empty code and
`Verify` never matches one, and `RandomCode` returns `""` for a non-positive
length instead of panicking.

### `cache/memory` reports use after close

Every method now takes a read lock and returns `cache.ErrClosed` once `Close`
has run, instead of reaching a half-torn-down ristretto. Concurrent
`Close`+`Set` used to park forever on `Wait`, and `Close`+`Del`/`DelAll` panicked
with "send on closed channel" — a routine sequence, since `task.Group` cancels
its members concurrently. `Close` is idempotent and still leaves an injected
ristretto cache open for its owner.

### `task.Group` always stops tasks whose Start ran

If every member's `Start` returned on its own, the group used to mark itself
stopped without calling `Stop`, and a later `Group.Stop` was a no-op. One-shot
tasks (migrations, warmup) never released. `Start` now runs `Stop` for every
task it launched, matching `Manager` and the `Task` contract. `boot.Run` of an
all-oneshot application therefore flushes in `Start` rather than skipping
cleanup.

`WithStartTimeout` is a new opt-in bound on non-final staged-group stages
only. A long-running last-stage `Start` (HTTP, workers) is exempt, and
`NewGroup` is a single stage so the option has no effect there. The error
wraps `ErrStartTimeout` and names the tasks still inside `Start`.

### `task.execute` names the task and stops logging provoked cancellation

Start/Stop errors are wrapped as `<identifier>: <err>` so `errors.Join` still
identifies the member. `errors.Is` / `As` are unchanged. A `context.Canceled`
that the runner itself provoked is no longer logged at Error — a graceful
shutdown no longer looks like every task crashed. A `Canceled` that arrives
while the run context is still live is still a failure and still logged.

### `task.Group.Stop` before `Start` is honoured

`Stop` returned `ErrGroupNotStarted` and did nothing if it arrived before
`Start` completed its state transition; the group then ran with no way to stop
it, because `Stop` is the only entry point and its channel does not exist until
`Start` creates it. `Stop` now records the request and returns nil, and `Start`
tears the group down immediately. `ErrGroupNotStarted` is removed. This also
stops a nested group from contributing a spurious error to a successful
cascading shutdown.

### `task.Manager` no longer discards wrapped cancellation errors

A task returning an error that wraps `context.Canceled` while its run context
was still live had that failure dropped: `Wait`, `GetTaskResult` and `StopTask`
all reported success. The guard now matches `Group`'s and only ignores
cancellation that this run actually caused.

### `mq/redis` `Consume` observes context cancellation

`BLPOP` was issued with an unlimited timeout, and go-redis applies no read
deadline to it, so cancelling the context did not interrupt a call already in
flight — a consumer parked on an empty queue leaked its goroutine for the life
of the process. It now polls with a bounded block and re-checks the context
between rounds, so cancellation is observed within about a second rather than
never.

### `qiniu` downloads report a real size and a real 404

`DownloadFile` read `ContentLength` from a field the SDK only fills after the
body has been streamed, so it was always 0 — which the HTTP layer turns into
`Content-Length: 0` and an empty response body for every download served
through this driver. It now stats the object for the size, as the `s3` driver
does, at the cost of one extra round trip. A missing key on the download path
answers with HTTP 404 rather than Qiniu's 612, so it is now mapped to
`storageerr.ErrorNotFound` instead of surfacing as a generic 500.

## Contract and robustness fixes

Lower-severity than the section above, but several change what callers observe.

### Cache

- **Byte caches now hold copies.** `mcache` and `memory` stored and returned the
  caller's own slice, while `redis` and `badgerdb` produced fresh ones. Reusing
  an encoding buffer after `Set`, or appending to a value from `Get`, therefore
  corrupted the cache on some backends and not others. Both in-process drivers
  now copy `[]byte`; `Core` documents the rule, and it applies only to byte
  slices — values of other types are still stored as given.
- **`CodecCache.MultiGet` no longer fails a whole batch** because one entry
  cannot be decoded. The bad entry is logged, deleted so the next read rebuilds
  it, and skipped. Previously a single value written under an older schema
  discarded every other result, and one stored without a TTL did so
  permanently, since the loader only rebuilds on a miss.
- **`nocache` implements `KeyLister`**, so `nscache.NSCache.DelAll` works over
  it instead of returning `ErrNotSupported`. Turning caching off by
  configuration used to break `DelAll` on a cache holding nothing.

### Lifecycle

- **`task.NewGroup` defaults its cleanup timeout to 30s**, matching
  `WithManagerCleanupTimeout`. It was unbounded, so a task whose `Stop` honoured
  its context but never returned left the group stopping — and `Start` blocked —
  forever. Pass `WithCleanupTimeout(0)` for the old behaviour.
- **`boot.WithShutdownTimeout` treats a non-positive duration as unbounded**,
  matching `task.WithCleanupTimeout`. It previously produced an already-expired
  context that skipped graceful shutdown entirely.

### Scheduler

- **Routing is by exact kind.** asynq's ServeMux falls back to longest-prefix
  matching, and the dispatcher resolved handlers from the matched pattern, so an
  unregistered kind was delivered to a handler registered under a prefix of it —
  `user.email.welcome.v2` landing in the handler for `user.email`.
- **A task with no handler is reported as a failure**, so asynq retries and then
  archives it, matching what the documentation always described. It was
  acknowledged as completed and discarded with only a log line. This also covers
  a task queued before its handler was unregistered, which now surfaces instead
  of vanishing.
- **`Start` returns when the run context is done and does not drain.** An
  earlier release called `Stop(WithoutCancel)` from `Start` so a cancelled
  parent would tear the runtime down, but that made Group/boot shutdown
  budgets unreachable. Call `Stop`. `boot.Run` and `task.Group` always do.
- **`Handle` rejects an empty or blank kind** instead of panicking inside asynq.
- **`Close` takes the same lock `Start` uses**, and a `Stop` that times out no
  longer cancels the shared run context, which used to abort handlers another
  concurrent `Stop` was still legitimately draining.
- `docs/scheduler.md` now states that periodic jobs require a single instance.
  Neither driver coordinates across processes, so N replicas run each job N
  times per tick.

### Messaging

- **`mq/redis` reports undecodable messages with their raw bytes** through
  `*redis.DecodeError`, and `TryConsume` returns `found=true` for them. It
  previously returned `found=false` with the element already popped and
  destroyed, so the payload was unrecoverable and the caller was told nothing
  had been waiting.
- **`memory.WithQueueSize` ignores sizes below 1.** Zero produced an unbuffered
  channel, and since `Broadcast` sends non-blockingly, an idle subscriber
  silently dropped most messages; negative panicked on first use.
- **`redis.PubSub.Subscribe` performs its round trip outside the lock**, so an
  unresponsive Redis no longer blocks `Close`/`UnsubscribeAll`. A `Subscribe`
  completing after `Close` now returns `redis.ErrPubSubClosed` rather than
  registering a connection nothing would close.

### Log

- **`InitWithBackends` with no usable backend keeps the current one** and warns
  on stderr, instead of installing a silent logger and discarding every
  subsequent line. Pass `NewNopBackend()` to discard deliberately.
- **`WrapBackendWithContextMerge` forwards `Close`**, so the documented release
  pattern reaches the wrapped backend's file handle.
- **`zapx.Sync` no longer fails because of the console sink.** Syncing stdout
  issues an fsync the kernel rejects for a pipe, which under a container or CI
  is what stdout always is, so `Sync` used to fail every time and mask a real
  file-sink failure.
- **`zapx.SlogHandler` carries the backend's logger name**, which the bridge
  dropped because zap keeps the name outside the Core.

### HTTP and storage

- **`WithRecover` re-panics on `http.ErrAbortHandler`**, letting net/http drop
  the connection as intended rather than logging a full stack trace and writing
  500 to a client that already disconnected.
- **The reverse proxy no longer truncates a response when the cache write
  fails.** Both sinks shared an `io.MultiWriter`, which fails as a whole, so a
  rejected cache write cut off the client mid-body while the 200 was already
  sent. Cache saves are now bounded by `WithSaveTimeout` (30s default) and both
  `With*ErrorHandler` options ignore a nil handler and have working defaults.
- **`kvcache.MoveFile` onto the same key is a no-op** instead of copy-then-delete,
  which reported success while destroying the object.
- **The one-time upload token is consumed with `GetDel`**, closing the window in
  which two concurrent requests could both redeem it.
- **`infra/sqlite` delegates to modernc's registered driver instance**, so
  package-level registrations (scalar functions, collations, connection hooks)
  apply. A zero-value driver carried none of them, and a missing custom
  collation quietly fell back to byte ordering.

### Captcha

- **Too many failed attempts freeze verification for 15 minutes instead of
  invalidating the outstanding codes.** Destroying them let anyone who knew a
  phone number wipe the code its owner had just received and, with the send
  limit blocking a resend, keep them locked out indefinitely at no cost.
- **The send limits are rolling windows.** A counter was only reset once it had
  already hit its limit, so a number used a couple of times a day accumulated
  across days until it was refused for a "daily" limit it never reached on any
  single day.

## Contract decisions

Where the repository previously held two answers to the same question, these
settle on one.

### `Close` waits for in-flight handlers

`mq`'s pubsub drivers returned from `Close`/`UnsubscribeAll` while handlers were
still executing (measured at ~30µs, with the handler mid-run), whereas
`scheduler` already committed to draining. Ordered shutdown needs the stricter
reading — but the ordering is not what the default group provides.
`boot.Application` is a plain `NewGroup`, and a group without stages stops all
its members concurrently; there is no guaranteed order between closing the
pubsub and closing the database or the log backend. An application that needs
the pubsub drained before the database and log backends close must arrange it
itself: nest a `NewStagedGroup` through `boot.NewApplication` and put the
pubsub in the **last** stage, since staged shutdown runs in reverse — the last
stage stops first. Either way, a `Close` that returned while handlers were
still executing pulled the database and log backends out from under a running
handler, so both drivers now wait. `mq.PubSub` documents it, and a shared
contract test covers them. `Close` can now block for as long as a handler runs
— a handler that never returns will stall shutdown, so bound them.

The redis driver's `UnsubscribeAll` waits for every consumer currently draining,
not only the topic's: go-redis signals a closed subscription solely by closing
its message channel, so per-topic accounting is not available. In teardown,
where `UnsubscribeAll` is used, that is the intended effect anyway.

### `TryConsume` reports failure through the error, not the bool

The documented rule ("when bool is false, error should be nil") was violated by
both drivers and by the contract test that pinned them, and following it
literally produced a loop that spun forever after `Close` while swallowing every
transport error. The contract now matches the implementations and Go's usual
`(T, bool, error)` shape: check the error first; the bool is meaningful only
when the error is nil. No driver behaviour changed — the documentation did.

### Encoded identifiers have exactly one spelling

`baseconv` decoding now rejects input the encoder could not have produced
(`ErrNonCanonical`): a trailing partial group whose padding bits are not zero,
and input long enough to carry a character that encodes nothing. `numconv`
requires the full eight-byte form, so `Base32ToInt64`/`Base62ToInt64` no longer
accept a value written without its leading zeros.

Previously `"5"` and `"00000005"` both decoded to 5, and two base32 strings
differing only in a discarded bit decoded identically — so distinct strings
denoted the same value and slipped past any deduplication, idempotency key or
cache lookup done on the string, which is what these identifiers exist for.
**Strings produced by this package are unaffected**; only hand-written or
externally generated forms that were never canonical stop decoding.

### `online` must be started

`online.Online` implements `core/task.Task` and requires `Start` for its storage
to stay bounded. The middleware only writes, and the backing cache reclaims an
expired entry only when that key is read again or the map is swept, so nothing
reclaimed anything: every key ever seen stayed resident, which for a client IP
or session id is unbounded. `Start` sweeps on a timer (`WithTrimInterval`,
default one minute), keeping the full-map scan off the request path. Return the
tracker from the application builder alongside the server. `NewOnline` now takes
options.

## Remaining contract decisions

### Object keys are normalized, and `UploadFile` returns the normalized key

`storage.NormalizeKey` defines one rule for every driver: a leading `/` is
dropped, repeated separators and `.` segments collapse, a trailing `/` is
removed, a `..` segment is rejected rather than resolved, and a key that
normalizes to nothing is rejected.

The drivers previously disagreed. `local` folded `"a/"` and `"d//x"` through
`filepath.Clean` but returned the caller's raw key from `UploadFile`, so a key
persisted in a database resolved here — it was normalized again on the way back
in — and 404'd on `s3` or `kvcache`, which kept keys verbatim. An empty key was
rejected by `local` while `kvcache` stored an object reachable only by the empty
string. **Persist the key `UploadFile` returns, not the one passed in.**

Listing prefixes are not keys and are unaffected: an empty prefix still means
"list everything", so `ListFiles` only strips a leading separator.

### Batch cache operations are documented as non-atomic

`Bulk` now states that `MultiSet`/`MultiDel` are not atomic: a non-nil error
means at least one entry failed, entries that succeeded may already be visible,
and nothing is rolled back. No driver changed. The contract follows what redis
can actually offer — a pipeline cannot revert commands already sent — so
compensating for a failed batch by deleting what it "would have" written is
unsafe. `badgerdb` and `mcache` remain atomic in practice; do not rely on it.

### `badgerdb.DelAll` no longer uses `DropAll`

It enumerates and deletes in batches, like `nscache.DelAll`. badger documents
`DropAll` as unsafe against concurrent reads — the caller must guarantee none are
in flight or it may panic — and that guarantee cannot be offered through
`Evictor`, a required member of the `Cache` interface that any holder of a
`cache.ByteCache` can call without knowing the driver underneath. Like the
nscache equivalent it is not atomic: a key written between the scan and the
delete survives.

### A log call cannot abort the goroutine that made it

Both backends recover around attribute rendering and emit an `attr_error` field
instead. Attribute values are arbitrary caller data, so a `Stringer` or
`MarshalJSON` of their own can panic; previously that propagated out of `Log` on
the zapx backend while the stdio backend degraded, so swapping backends changed
whether an application survived its own logging. For zapx the recover has to
wrap the write, because `zap.Any` is lazy and the value is encoded at write time.

`formatAny` also refuses to format a value that is cyclic or nested deeper than
32 levels, reporting it instead. `fmt.Sprint` has no cycle detection, and the
resulting stack overflow is fatal to the process and cannot be recovered — it
has to be prevented before formatting starts.

### `local` writes survive power loss

`writeFileAtomic` now fsyncs the parent directory after the rename. Syncing the
file only covers its contents; the directory entry could still be in the page
cache, so the durability the doc comment promised held against a process crash
but not against power loss.

### `search.Result.Total` is documented as an estimate

It always was one — Meilisearch clamps it to `pagination.maxTotalHits`, 1000 by
default — but the field was documented as an exact count, so paginating on it
ran off the end of the results. No behaviour changed.

## Known issue requiring an upstream fix

`WithRecover` can emit two JSON documents in one response when a handler panics
*after* it has already written its body: the second write appends rather than
replaces, leaving a 200 with a concatenated payload. This cannot be fixed here.
`httpx.Responder` documents that a response becomes committed once written, but
`httpx.Context` exposes no way to query that state, so `AbortWithJsonError` has
no way to know it must stay silent. The fix belongs in `go-sphere/httpx`.

## Known limitations (unchanged since v0.0.3, not regressions)

- `badgerdb.MultiSet`/`MultiSetWithTTL` write every entry in one transaction and
  fail with `ErrTxnTooBig` on large batches (reproduced at 20000 x 1KiB).
- `mcache.Map` has no size bound and no background eviction. Expired entries are
  reclaimed only when that key is accessed again or `Count`/`Trim` is called.
- `badgerdb.Database` never runs `RunValueLogGC` and does not expose it, so the
  value log is not reclaimed.
- `cache.GetEx`/`GetObjectEx`/`GetJsonEx` ignore the error from writing the
  freshly built value back into the cache.
- The `redis` driver takes a `*redis.Client`, so Cluster and Sentinel
  deployments are not supported.
