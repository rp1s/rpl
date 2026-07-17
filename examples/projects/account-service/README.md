# Account Service

This project demonstrates a validation-first application service backed by the
SQL API generated from one RPL model. It is intentionally small enough to read
in one sitting but includes the same separation used in a production service:
schema, generated persistence, handwritten business logic, executable, and
tests.

## What Is Generated

The schema in `src/main.rpl` produces:

```text
generated/account/
├── model.gen.go
├── meta.gen.go
├── validation/
│   └── validation.gen.go
└── sql/
    ├── queries.gen.go
    ├── scan.gen.go
    └── schema.gen.go
```

The SQL package exposes:

- `Executor`, compatible with `*database/sql.DB` and `*database/sql.Tx`;
- `NewStore`, `Init`, `Create`, `Get`, `Update`, `Upsert`, `Delete`, `List`, and `Search`;
- typed columns such as `ColumnEmail`;
- `Where` and `And` helpers that reject unknown SQL filter columns;
- SQLite DDL, placeholders, indexes, defaults, and scan conversion.

The validation package exposes `Validate(account)` for the first combined error
and `Errors(account)` when a caller needs every field error.

## Generate and Test

From this directory:

```bash
rpl run src/main.rpl out generated
go test ./...
go run ./cmd/check
```

From the repository root without installing RPL:

```bash
./build/$(go env GOOS)-$(go env GOARCH)/rpl \
  run examples/projects/account-service/src/main.rpl \
  out examples/projects/account-service/generated
go -C examples/projects/account-service test ./...
```

The tests use a recording implementation of the generated `sql.Executor`.
That keeps CI independent of a database while proving that schema creation,
validation, and INSERT generation are wired together.

## Application Flow

`internal/accounts.Service.Register` is deliberately handwritten:

1. validate the generated `Account` value;
2. stop before storage when validation fails;
3. call the generated store for a valid value.

The application depends on stable generated interfaces rather than on SQL
string constants or field reflection. `FindByEmail` also shows how a typed
column becomes a safe generated filter.

## Connect a Real SQLite Database

Add the SQLite driver preferred by your application, open a `*sql.DB`, and pass
it to `accounts.New`:

```go
db, err := sql.Open("sqlite", "file:accounts.db")
if err != nil {
    return err
}
service := accounts.New(db)
if err := service.Init(context.Background()); err != nil {
    return err
}
```

The driver is not pinned in this example because driver choice, CGO policy, and
deployment platform belong to the consuming application. The generated API uses
only `database/sql`.

## Exercises

- Add `DeletedAt ?time.Time` and decide whether it belongs in normal searches.
- Add a second unique field, regenerate, and inspect the upsert conflict clause.
- Replace the recording executor with an in-memory SQLite integration test.
- Add a transport attr and expose the same service through gRPC or `os.bin`.

Never edit `generated/` manually. Delete it at any time and recreate it from
`src/main.rpl`.
