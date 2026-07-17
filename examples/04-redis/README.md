# Redis examples

`rpl:redis` generates deterministic keys plus hash serialization and decoding.
It intentionally leaves the choice of Redis client to the application.

| Example | Features |
| --- | --- |
| `01-session-cache.rpl` | explicit TTL, unique keys, time and defaults |
| `02-redis-ignore-fields.rpl` | attr-local and shared `@ignore` forms |
| `03-composite-cache-key.rpl` | multi-part key, scalar/list/nested hash encoding |
| `04-optional-and-fallback-key.rpl` | optional values and automatic `UserID` key selection |

Every `unique: true` field becomes part of `RedisKey()`. Without explicit
unique fields, `ID`/`*ID` fields are preferred, then the first active field.
Lists and nested models use JSON inside the Redis hash; scalar and time values
use stable textual encodings.

```go
key := value.RedisKey()
hash, err := value.RedisHash()
err = decoded.ApplyRedisHash(hash)
```
