# rpl:sql

`rpl:sql` generates PostgreSQL or SQLite DDL, scanners, CRUD/query helpers,
and a small Go-style `Store` for every model.

## Schema

```rpl
@sql(db: "postgres", table: "users")
model User {
    Id int @sql(column: "id", primaryKey: true)
    Email string @sql(unique: true, index: true)
    Name string @sql(index: true)
    CreatedAt time.Time @sql(default: "now")
    UpdatedAt time.Time @sql(default: "now", updatedAt: true)
    Internal string @sql(ignore: true)
}
```

Model arguments:

- `db`: `postgres` (default) or `sqlite`;
- `table`: explicit table name; defaults to the snake_case model name.

Field arguments:

- `column`: explicit column name;
- `primaryKey`: marks a primary-key field; several fields form a composite key;
- `unique`: creates a standalone unique constraint;
- `index`: creates a non-unique index when the field is not already a key;
- `default`: SQL default (`now` becomes `CURRENT_TIMESTAMP` for `time.Time`);
- `updatedAt`: refreshes a `time.Time` value during `Update` and `Upsert`;
- `ignore`: excludes the field from SQL storage.

Identifiers are validated and quoted. Primary-key columns are excluded from the
generated `UPDATE SET` list. `Upsert` uses the primary key when present; without
a primary key it uses the first `unique` field as its conflict target.

## Generated Go API

The package keeps the functional API:

```go
err := usersql.Init(ctx, db)
user, err := usersql.Get(ctx, db, usersql.Where(usersql.ColumnId, id))
err = usersql.Update(ctx, db, user, usersql.Where(usersql.ColumnId, id))
```

For repeated operations, use `Store`. Both `*sql.DB` and `*sql.Tx` implement the
generated `Executor` interface:

```go
store := usersql.NewStore(db)
if err := store.Init(ctx); err != nil {
    return err
}

filters := usersql.And(
    usersql.Where(usersql.ColumnEmail, email),
    usersql.Where(usersql.ColumnName, name),
)
user, err := store.Get(ctx, filters)
```

`Init` is intentionally bootstrap-only: it executes `CREATE TABLE IF NOT EXISTS`
and `CREATE INDEX IF NOT EXISTS`. It is not a destructive migration engine.
