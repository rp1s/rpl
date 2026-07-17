# SQL examples

`rpl:sql` generates PostgreSQL or SQLite schema constants, typed columns,
filters, scans, CRUD functions, upsert logic, and a `Store` facade.

| Example | Features |
| --- | --- |
| `01-user-storage.rpl` | PostgreSQL, unique/indexed fields, validation, timestamps |
| `02-sql-ignore-and-defaults.rpl` | defaults and two forms of SQL exclusion |
| `03-sqlite-storage.rpl` | SQLite plus list and nested-model JSON columns |
| `04-composite-primary-key.rpl` | three-column primary key, custom names, typed defaults |
| `05-json-and-custom-columns.rpl` | custom columns, optional values, list/model JSON encoding |

Several `primaryKey: true` fields form a composite conflict target for upsert.
When no primary key exists, the first unique field is used. Lists and nested
models are stored as JSON text while scalar fields retain native SQL types.

Generated consumers can use either functions or the facade:

```go
store := sqlstore.NewStore(db)
err := store.Upsert(ctx, order)
items, err := store.List(ctx, 50, 0)
```
