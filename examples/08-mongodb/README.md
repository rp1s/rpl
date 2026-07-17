# MongoDB examples

`rpl:mongodb` generates collection/index declarations, BSON adapters, safe
search/sort helpers, CRUD, count, update, watch, and bulk-oriented helpers.

| Example | Features |
| --- | --- |
| `01-user-store.rpl` | ObjectID, indexes, search/sort allowlists, timestamps |
| `02-sparse-profile-store.rpl` | optional fields, sparse unique index, custom BSON names |
| `03-search-sort-and-defaults.rpl` | catalog search, compound index, descending key, timestamp defaults |

`objectId: true` converts a string field to/from MongoDB `primitive.ObjectID`.
`search: true` and `sort: true` build explicit allowlists so callers cannot
inject arbitrary document paths. `omitempty` affects BSON documents, while
`sparse` configures the generated MongoDB index.

Fields with the same `indexGroup` form one compound index in declaration
order. `indexOrder` accepts `1` (ascending, the default) or `-1` (descending).
If a grouped field sets `unique` or `sparse`, that option applies to the whole
compound index.

Generated helpers include index initialization, insert/find/list/count,
replace/update/delete, distinct, aggregation, and change-stream watch APIs.
