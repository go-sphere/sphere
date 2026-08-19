# Behaviour Changes since v0.0.3

`api-incompatibilities.txt` is produced by `apidiff` and only sees **signature**
changes. This file records changes where the signature is identical but the
runtime behaviour is not — exactly the changes a compiler cannot catch and an
upgrade can silently break.

Scope: this pass covers `cache/` only. The other top-level packages have not
been reviewed for behavioural changes yet.

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
