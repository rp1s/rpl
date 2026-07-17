# WebAssembly Component examples

- `01-rust-component.rpl` generates a WIT world, deny-by-default manifest,
  Rust `wit-bindgen` guest, and typed Wasmtime host bindings.
- `02-contract-only.rpl` generates only the portable contract, manifest, and
  normalized IR snapshot.

Run:

```bash
rpl run examples/11-wasm/01-rust-component.rpl out /tmp/rpl-wasm
```

Inspect `/tmp/rpl-wasm/wasm/user-service`. Native FFI is a separate concern;
use `rpl:ffi` when the output must be a process-native dynamic library rather
than a sandboxed WebAssembly component.
