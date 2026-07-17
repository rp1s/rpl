# FFI examples

`rpl:ffi` turns model methods into a versioned in-process ABI. A generated C
header is the source of truth; C and Rust can implement the server, while Go,
Python, C, and Rust clients call the same method names and JSON contracts.

| Example | Features |
| --- | --- |
| `01-rust-server-all-clients.rpl` | Rust trait server, all clients, model/list/optional fields, multi-value response |
| `02-c-server-selected-clients.rpl` | C callback-vtable server and selected Go/Python clients |

The boundary uses tiny C-compatible views and owned buffers. Method payloads
are UTF-8 JSON, which avoids sharing language-specific object layouts across
the ABI. Scalar fields still receive explicit C types in the generated model
struct; lists and nested values use JSON views.

```rpl
target(lang: ffi)

@ffi(server: "rust", clients: "go,go:purego,python,c,rust", library: "calculator")
model CalculatorService {
    func Add (left int64, right int64) return (int64)
}
```

The artifact-only `ffi` target intentionally does not generate a root
`model.gen.go`. Choosing a Go client only adds the self-contained client model
and request/response types under `ffi/go`; it does not change the Rust server.

For a Go binary without cgo, include `go:purego` in `clients`, install
`github.com/ebitengine/purego@v0.10.1`, and build with
`CGO_ENABLED=0 -tags rpl_ffi_purego`. The generated
`OpenPureGoFromFactory` loader supports a Rust or C shared library through the
same pointer-only ABI. Plain `go` selects the cgo bridge.

Generated output includes `calculator_service.h`, `schema.json`, the selected
server implementation, and client packages. See the full
[`rpl:ffi` guide](../../src/attrs/rpl:ffi/README.md) for ownership and build
details.
