# Session Cache

This project treats Redis as a serialization boundary. RPL generates stable key
construction and hash conversion while the handwritten package decides how a
Redis client should execute commands, retries, tracing, and expiration.

## Generated Contract

`src/main.rpl` creates a `Session` with:

- `RedisKey() string` — produces `sessions:<id>`;
- `RedisHash() (map[string]string, error)` — serializes scalars, lists, and times;
- `ApplyRedisHash(map[string]string) error` — restores typed values;
- validation for IDs, tokens, and user IDs;
- std metadata and the model comment.

`DebugNote` has `@redis(ignore: true)`. The round-trip test proves that it does
not leak into the persisted hash. `Roles` demonstrates JSON conversion inside a
Redis string field, and timestamps use RFC 3339 with nanoseconds.

## Generate and Run

```bash
rpl run src/main.rpl out generated
go test ./...
go run ./cmd/demo
```

Expected demo output has this shape:

```text
SET sessions:session-demo fields=6 ttl=3600
```

The exact field count changes when the schema changes. The key prefix and TTL
come from `@redis(table: "sessions", ttl: 3600)`.

## Why the Generated Code Does Not Open Redis

The Redis attr generates a storage representation, not a network client. This
keeps generated models independent of a specific Redis library and makes the
same code usable with standalone Redis, Sentinel, Cluster, test doubles, and
managed providers.

A real adapter usually performs these operations:

```go
entry, err := cache.Encode(session)
if err != nil {
    return err
}
if err := client.HSet(ctx, entry.Key, entry.Values).Err(); err != nil {
    return err
}
return client.Expire(ctx, entry.Key, time.Duration(entry.TTLSeconds)*time.Second).Err()
```

Reading reverses the flow: `HGetAll`, then `cache.Decode(values)`.

## Test Coverage

`internal/cache/codec_test.go` verifies:

- validation before encoding;
- stable key construction;
- explicit TTL propagation;
- exclusion of ignored fields;
- list and timestamp serialization;
- a complete hash round trip.

The repository-level `make test-projects` removes generated output, regenerates
the module in a temporary directory, and runs this test against the new files.

## Exercises

- Add a nullable field and inspect its absent/present hash behavior.
- Make `UserId` part of the key by adding `@redis(unique: true)`.
- Add a Redis client adapter behind a small interface and test command errors.
- Add a schema version field to support rolling migrations of cached data.
