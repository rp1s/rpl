# std examples

`rpl:std` supplies the language-level metadata attrs used by other generators.

| Example | Features |
| --- | --- |
| `01-comment-group-ignore.rpl` | model/field comments, one derived group, selective ignore |
| `02-groups-for-derived-models.rpl` | generated `UserReq` and `UserPublic` types referenced by another model |
| `03-selective-ignore.rpl` | several projections from one model and per-runtime privacy boundaries |
| `04-derived-command-models.rpl` | create, patch, and public DTOs combined with validation |

The important distinction is that `@ignore("sql")` only removes a field from
that generator. It does not remove the field from the base Go model. `@group`
can occur more than once on a field, allowing one source field to participate
in several generated projections.

```rpl
Email string @group("create") @group("patch")
PasswordHash string @ignore("grpc", "transport")
```
