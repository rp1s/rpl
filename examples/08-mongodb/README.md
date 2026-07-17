# MongoDB examples

`rpl:mongodb` generates collection/index declarations, BSON adapters, safe
search/sort helpers, CRUD, count, update, watch, and bulk-oriented helpers.

| Example | Features |
| --- | --- |
| `01-user-store.rpl` | ObjectID, indexes, search/sort allowlists, timestamps |
| `02-sparse-profile-store.rpl` | optional fields, sparse unique index, custom BSON names |
| `03-search-sort-and-defaults.rpl` | catalog search, several indexes, timestamp defaults |

`objectId: true` converts a string field to/from MongoDB `primitive.ObjectID`.
`search: true` and `sort: true` build explicit allowlists so callers cannot
inject arbitrary document paths. `omitempty` affects BSON documents, while
`sparse` configures the generated MongoDB index.

Generated helpers include index initialization, insert/find/list/count,
replace/update/delete, distinct, aggregation, and change-stream watch APIs.
