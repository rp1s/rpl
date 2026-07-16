# Showcase Package

This folder is the densest built-in runtime example in the repository.

Start with:

- `main.rpl` for the large `User` model plus `Session` and grouped models
- `realtime.rpl` for `@grpc.Model()`, `@grpc(subject: "id")`, and `inside`
- `extensions.rpl` for top-level field/model extensions
- `support.rpl` for a reusable nested model in the same package

`main.rpl: User` is the most feature-rich single model example in the tree.
The whole package together demonstrates every built-in runtime:

- `rpl:grpc`
- `rpl:sql`
- `rpl:redis`
- `rpl:std`
- `rpl:validate`
