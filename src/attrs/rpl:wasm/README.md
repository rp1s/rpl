# `rpl:wasm`

`rpl:wasm` turns an RPL model and its methods into a WebAssembly Component
Model contract. It is deliberately separate from `rpl:ffi`: WASM uses WIT and
the Canonical ABI, while FFI targets native shared libraries through C ABI.

```rpl
target(lang: ffi)

attrs (
    "rpl:wasm"
)

@wasm(
    wit: "example:users@1.0.0",
    world: "user-plugin",
    interface: "user-service",
    guest: "rust",
    hosts: "rust",
)
model UserService {
    Id uint64
    Name string
    Email string?
    Tags []string

    func GetUser (id uint64) return (UserService?, error)
    func Save (user UserService) return (uint64, error)
    func List return ([]UserService, error)
}
```

Generated tree:

```text
wasm/user-service/
├── wit/world.wit
├── plugin.toml
├── schema.json
├── README.md
├── guest/rust/
│   ├── Cargo.toml
│   ├── wit/world.wit
│   └── src/lib.rs
└── host/rust/
    ├── Cargo.toml
    ├── wit/world.wit
    └── src/lib.rs
```

## RPL to WIT

| RPL | WIT |
| --- | --- |
| `bool` | `bool` |
| `int8..int64` | `s8..s64` |
| `uint8..uint64` | `u8..u64` |
| `float32`, `float64` | `f32`, `f64` |
| `string` | `string` |
| `[]T` | `list<T>` |
| `T?` | `option<T>` |
| an RPL model | `record` |
| final method return `error` | `result<T, string>` |
| several successful returns | `tuple<...>` |

`time.Time`, recursive model graphs, callbacks, maps, raw pointers, resources,
and custom error variants are rejected by the MVP. RPL does not yet have an
enum/variant declaration, so business errors temporarily use `string`. Once
the core syntax gains sum types, this plugin can lower them to WIT `variant`
without changing its host/guest boundary.

## Isolation

The generated manifest denies filesystem, network, environment, and stdio by
default. It records memory, timeout, and fuel limits. The generated Wasmtime
host applies a store memory limiter, enables fuel and epoch interruption, and
exposes an interruption hook. The application embedding the host must connect
its wall-clock timer to that hook before treating timeout as enforced.

## Go status

Go guest and Go host are not accepted as `guest`/`hosts` values yet. The
Component Model ecosystem has Go binding work, but RPL only advertises a
language after a generated component and host pass an automated end-to-end
test in this repository.
