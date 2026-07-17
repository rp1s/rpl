# rpl:redis

`rpl:redis` generates deterministic keys, hash serialization/deserialization,
TTL metadata, and storage helpers without fixing the application to one Redis
client library.

```rpl
@redis(db: "cache", table: "sessions", ttl: 3600)
model Session {
    TenantID string @redis(unique: true)
    ID string @redis(unique: true)
    Active bool @redis(name: "is_active", default: "true")
    Attempts int @redis(name: "attempt_count", default: "0")
    ExpiresAt time.Time @redis(default: "now")
}
```

Model arguments are `db`, `table`, and TTL in seconds. Field arguments are:

- `name`: stored Redis hash key;
- `unique`: include the field in the deterministic composite model key;
- `default`: value applied when the hash key is absent;
- `ignore`: exclude the field from Redis generation.

Defaults are validated for the target field type. Scalar and time fields use
stable textual encodings; lists and nested models use JSON. Duplicate custom
hash names are rejected during analysis.
