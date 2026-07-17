# gRPC examples

`rpl:grpc` generates protobuf messages, service definitions, Go protobuf code,
typed clients, servers, and model conversion helpers.

| Example | Features |
| --- | --- |
| `01-basic-service.rpl` | automatic CRUD service with an explicit ID |
| `02-classic-custom-methods.rpl` | stateless custom RPC methods and parameters |
| `03-model-bound-methods-by-model.rpl` | model subject inherited by methods |
| `04-model-bound-methods-by-id.rpl` | ID subject inherited by methods |
| `05-inside-and-field-methods.rpl` | local-only external field and field methods |
| `06-nested-messages-and-ignore.rpl` | nested/repeated messages, multiple results, excluded secrets |
| `07-method-subject-overrides.rpl` | classic, ID-bound, model-bound, and ignored methods together |

Use classic methods for application-wide operations. Add `@grpc.Model()` when
an operation belongs to one model instance, then choose `subject: "id"` or
`subject: "model"`. Method-level subject settings override model defaults.
External Go values that cannot cross protobuf can stay local with
`@grpc.Inside()`.
