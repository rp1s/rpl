# Native FFI and WebAssembly plugins

RPL has two code-generation attrs for in-process cross-language boundaries.
They share the resolved RPL schema but deliberately do not share an ABI.

## Decision

| Attr | Boundary | Contract | Primary use |
| --- | --- | --- | --- |
| `rpl:ffi` | native shared library | generated C ABI | minimum overhead and integration with native code |
| `rpl:wasm` | sandboxed WASM component | WIT + Canonical ABI | portable, capability-limited application plugins |

The long standalone CLI and separate IDLs proposed in the original design
notes do not fit this repository. RPL already owns parsing, semantic analysis,
cross-file model resolution, diagnostics, editor integration, plugin discovery,
and deterministic output planning. Both features therefore remain SDK v2 attrs
under `src/attrs`, are bundled into normal releases, and use `.rpl` as their
source language.

## Shared front end

```text
.rpl source
  -> RPL parser and semantic analysis
  -> SDK Model/Method/TypeRef graph
  -> attr-specific validation and lowering
       -> rpl:ffi native ABI IR -> C/Rust/Go/Python code
       -> rpl:wasm WIT IR       -> WIT/wit-bindgen/Wasmtime code
```

Only the resolved schema graph is shared. Ownership, error representation,
packaging, compatibility rules, and runtimes belong to the individual attr.

## `rpl:ffi` direction

The stable native design uses method-specific C request/result structs,
fixed-width scalars, pointer-length views for borrowed input, owned buffers for
output, explicit presence fields for nullable values, status codes, and
creator-owned free functions. Panics must be caught at the server adapter and
converted into an ABI status. Go chooses the loader through `clients`:
`go` generates cgo, while `go:purego` generates the cgo-free loader.

The pre-existing implementation still has a compatibility JSON dispatch path
inside its method trampoline. That path is not the desired final native ABI;
it must be replaced incrementally by method-specific lowered structs while
preserving generated symbol versioning. The public docs must not describe that
compatibility path as zero-serialization FFI.

## `rpl:wasm` MVP

`rpl:wasm` never lowers strings or lists to handwritten pointers. It emits WIT
and delegates lowering to the Component Model Canonical ABI. Its current
verified target is a Rust guest scaffold using `wit-bindgen` and a Rust host
binding using Wasmtime. It also emits a versioned IR snapshot and a strict
manifest with denied capabilities and declared resource limits.

The generated host turns component/world mismatch into a load error, applies a
store memory limiter, enables fuel and epoch interruption, and exposes
Wasmtime's generated typed binding. The embedding application remains
responsible for wiring the declared wall-clock timeout to epoch interruption.

## Error model

Today RPL methods already support a final Go-style `error` result. For WASM it
lowers to WIT `result<T, string>`; runtime traps remain Wasmtime errors and are
not confused with that business result. For native FFI it lowers to a stable
status plus owned error payload. A future RPL `error`/`variant` declaration can
replace strings with structured WIT variants and structured FFI error data.

## Compatibility

The WIT package identity is explicit (`namespace:name@semver`). The generated
`schema.json` is deterministic and is the input for a later `rpl wasm compat`
command. Removing or renaming a function, changing parameters/results, making
an optional value required, or changing a record field is breaking. Packaging,
signature verification, registry installation, and `resource` handles are
post-MVP work and should be added to the RPL CLI only after the attr's generated
vertical slice is covered end to end.
