# rpl:ffi

`rpl:ffi` generates a versioned C ABI from an RPL model and its methods. The C
header is language-neutral; the same contract can be implemented by a C or
Rust server and consumed by Go, Python, C, or Rust.

## Schema

```rpl
target(lang: ffi)

attrs (
    "rpl:ffi"
)

@ffi(
    server: "rust",
    clients: "go,python,c,rust",
    library: "calculator",
    prefix: "calculator",
    abiVersion: 1,
)
model CalculatorService {
    ID int64
    Name string?

    func Add (left int64, right int64) return (int64)
    func Stats return (int64, float64)
    func InternalReset @ffi(ignore: true)
}
```

Use `target(lang: ffi)` for a pure ABI project. It is an artifact-only target:
the compiler keeps the per-model output directory but does not generate an
unrelated host `model.gen.go`. The selected clients remain self-contained, so
a Go client still receives its types under `ffi/go` even when the server is
Rust. Use `target(lang: golang)` only when the same RPL model is also intended
to be a normal Go domain model.

Model arguments:

- `server`: `c`, `rust`, `c,rust`, or `none`; default is `rust`;
- `clients`: comma-separated `go`, `python`, `c`, and `rust`; default is all;
- `library`: native library link/load name;
- `prefix`: stable C symbol prefix; defaults to the snake_case model name;
- `abiVersion`: positive ABI version exposed in the header and libraries.

Fields and methods accept `name` for a stable wire name and `ignore: true` for
exclusion. Renaming an RPL identifier while retaining `@ffi(name: "...")`
keeps the external contract stable.

## Generated tree

```text
ffi/
├── calculator_service.h       canonical ABI, model layout, vtable, symbols
├── schema.json                machine-readable fields, methods, params, returns
├── c/
│   ├── calculator_service_server.c
│   └── calculator_service_client.c
├── go/
│   ├── client.gen.go          typed requests and NativeABI abstraction
│   ├── native_cgo.gen.go      optional cgo bridge, enabled by rpl_ffi_cgo
│   ├── native_purego.gen.go   cgo-free dynamic bridge, enabled by rpl_ffi_purego
│   ├── native_purego_unix.gen.go
│   └── native_purego_windows.gen.go
├── python/
│   └── calculator_service_ffi.py
└── rust/
    ├── Cargo.toml
    └── src/lib.rs             typed trait server and/or native client
```

Only selected languages are emitted. The header and `schema.json` are always
generated so an additional language binding can be implemented independently.
Selecting `c,rust` emits two alternative server implementations with the same
symbols; build one of them into a particular native library, not both together.

## ABI and ownership

Every call has the same stable shape:

```c
int32_t calculator_ffi_server_call(
    calculator_ffi_server *server,
    calculator_ffi_view method,
    calculator_ffi_view request_json,
    calculator_ffi_buffer *response_json,
    calculator_ffi_buffer *error_message
);
```

The header also exposes pointer-only `*_ffi_server_call_bytes` and
`*_ffi_buffer_free_bytes` entry points. They preserve the same ownership and
JSON contract without passing a C struct by value, which makes the ABI safe to
register through pure-Go foreign-function runtimes.

Views are borrowed for the duration of the call. Response and error buffers
are owned by the caller and must be released exactly once with
`calculator_ffi_buffer_free`. Status `0` is success; non-zero statuses separate
invalid input, unknown/unimplemented methods, decoding errors, service errors,
and Rust panics.

Generated clients compare the library's exported ABI version before the first
call. A mismatch is rejected instead of attempting to interpret incompatible
buffers.

Method parameters are encoded as a JSON object using their snake_case or
configured wire names. A single return value is encoded directly. Multiple
return values use `value1`, `value2`, and so on. This wire format lets nested
models, optionals, lists, and future fields cross the ABI without relying on a
compiler-specific struct layout.

## Rust server

The Rust crate generates request/response structs and an `FFIService` trait:

```rust
struct Calculator;

impl FFIService for Calculator {
    fn add(&mut self, request: AddRequest) -> Result<i64, FFIError> {
        Ok(request.left + request.right)
    }
}

let handle = into_raw_server(Calculator);
```

The exported call catches panics before they cross the C boundary. The owner of
the handle eventually calls the generated destroy symbol. To let external
clients construct the service automatically, export a small application-level
factory such as `calculator_ffi_server_default` that returns this handle.

## C server

The C server accepts a generated callback vtable. Each method callback receives
its canonical JSON request and writes either a response or error buffer:

```c
calculator_ffi_service_vtable service = {
    .abi_version = CALCULATOR_FFI_ABI_VERSION,
    .context = app,
    .method_add = app_add,
    .drop_context = app_destroy,
};
calculator_ffi_server *server = calculator_ffi_server_create(service);
```

Use the generated `*_ffi_buffer_copy` helper for callback results so the
matching `*_ffi_buffer_free` function can release them safely.

## Go client

The portable Go API depends on `NativeABI`, making it easy to test without a
native library. The recommended native mode uses
[`ebitengine/purego`](https://github.com/ebitengine/purego) and does not require
a C compiler while building the Go application:

```bash
go get github.com/ebitengine/purego@v0.10.1
CGO_ENABLED=0 go build -tags rpl_ffi_purego ./...
```

```go
native, err := ffigo.OpenPureGoFromFactory("./libcalculator.so", "")
if err != nil {
    return err
}
defer native.Close()

client := ffigo.NewClient(native)
sum, err := client.Add(ctx, ffigo.AddRequest{Left: 2, Right: 3})
```

An empty factory name selects `<prefix>_ffi_server_default`. Pass an explicit
library path in production; `DefaultPureGoLibraryPath` returns the conventional
name for the current OS. The generated loader uses `dlopen` on Unix-like
systems and the Windows DLL API on Windows. `PureGoNative.Close` destroys a
factory-owned server and unloads the library; a handle passed to `OpenPureGo`
remains owned by the caller.

The cgo implementation remains available as an alternative:

```bash
go build -tags rpl_ffi_cgo ./...
```

```go
native := ffigo.NewCGONative(serverHandle)
client := ffigo.NewClient(native)
sum, err := client.Add(ctx, ffigo.AddRequest{Left: 2, Right: 3})
```

The generated cgo file links `-l<library>`. Supply normal compiler/linker search
paths through `CGO_CFLAGS` and `CGO_LDFLAGS` when the library is not installed
in a default location.

## Python client

Python uses `ctypes` and accepts an existing server handle:

```python
client = Client("./libcalculator.dylib", server_handle)
value = client.add(left=2, right=3)
```

`Client.from_factory` can call an application-exported factory symbol. The
default expected name is `<prefix>_ffi_server_default`.

## Compatibility rules

- bump `abiVersion` when C symbols, buffer layout, or ownership changes;
- keep `prefix`, method wire names, and field wire names stable;
- adding an optional JSON field is normally backward compatible;
- removing/renaming a method or changing a parameter/return type is breaking;
- never pass a language-owned pointer beyond the documented call lifetime;
- do not allow a panic, C++ exception, or Python exception across the ABI.
