# FFI service project

This project generates one native ABI from `CalculatorService`: a canonical C
header, Rust trait server, and Go/Python/C/Rust clients.

The schema uses `target(lang: ffi)`, so no unrelated Go host model is emitted.
The Go files under `ffi/go` are the explicitly selected client binding only.

```bash
rpl run src/main.rpl out generated
go test ./...
cargo test --manifest-path generated/calculator_service/ffi/rust/Cargo.toml
```

The Go test uses the generated `NativeABI` seam, so domain/client behavior is
testable without loading a dynamic library. For a cgo-free production client,
build with `CGO_ENABLED=0 -tags rpl_ffi_purego` and open the Rust library with
`OpenPureGoFromFactory`. The alternative cgo bridge remains available through
`-tags rpl_ffi_cgo` and `CGO_LDFLAGS`.

The generated Rust crate contains the `FFIService` trait. Implement it, create
a raw handle with `into_raw_server`, and optionally export an application
factory named `calculator_ffi_server_default` for Python or other dynamically
loaded clients.

The ABI contract deliberately owns only byte buffers. Requests and responses
use JSON described by `ffi/schema.json`, while buffer allocation/freeing stays
inside the server library that created the memory.
