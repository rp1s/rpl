# RPL Examples

This directory is a working cookbook for RPL.

If you want one big entry point first, start with:

- `99-showcase/main.rpl` for the largest package-level showcase
- `99-showcase/realtime.rpl` for `grpc.Model()`, `subject`, and `inside`

The closest "single rich model" example is `99-showcase/main.rpl: User`.
The full showcase package uses almost every built-in runtime across several models,
because some attrs are model-only, some are field-only, and some only make
sense on specific transports.

## Folders

- `00-syntax` - core language syntax without much runtime logic
- `01-std` - `@comment`, `@group`, `@ignore`
- `02-validate` - numeric, string, time, URL, phone, email, hash validation
- `03-sql` - SQL storage examples, including SQLite
- `04-redis` - Redis cache/storage examples
- `05-grpc` - classic and model-bound gRPC styles
- `06-multifile` - one package split into multiple `.rpl` files
- `07-imports` - importing another `.rpl` file
- `08-mongodb` - MongoDB collections, indexes, search, and CRUD helpers
- `09-transport` - local stdin/stdout process transport with generated shell client/server
- `99-showcase` - large end-to-end examples

## Run An Example

From the repository root:

```bash
go -C src run ./cmd run ../examples/05-grpc/01-basic-service.rpl out /tmp/rpl-out
go -C src run ./cmd run ../examples/09-transport/01-os-bin-service.rpl out /tmp/rpl-transport
```

For package-based examples, run the main file in that folder:

```bash
go -C src run ./cmd run ../examples/06-multifile/main.rpl out /tmp/rpl-multifile
go -C src run ./cmd run ../examples/99-showcase/main.rpl out /tmp/rpl-showcase
```

## Notes

- The examples prefer explicit attrs like `@grpc()` and `@sql(index: true)` so
  the syntax is easier to copy.
- SQL examples use `@sql(primaryKey: true)` for stable updates and upserts; the
  generated package also exposes typed columns, `Where`/`And`, and `NewStore`.
- Some examples use standard library imports such as `time` or `net/http`.
- The `grpc` examples show both classic custom methods and the explicit
  instance-style mode enabled by `@grpc.Model()`.
- The `transport` examples use `@transport(os.bin)` to generate a local shell
  transport over stdin/stdout without HTTP.
