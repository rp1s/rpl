# Transport examples

`rpl:transport` keeps one domain service interface and generates adapters for
local processes, HTTP, Unix sockets, NATS, Kafka, and WebSocket.

| Example | Features |
| --- | --- |
| `01-os-bin-service.rpl` | stdin/stdout child-process IPC |
| `02-http-service.rpl` | `net/http` handler and typed client |
| `03-unix-socket-service.rpl` | concurrent Unix socket server/client |
| `04-nats-service.rpl` | request/reply broker abstraction |
| `05-kafka-service.rpl` | correlation-aware Kafka RPC bridge |
| `06-websocket-service.rpl` | library-neutral connection/upgrader interfaces |
| `07-multi-adapter-service.rpl` | all adapters around one service contract |
| `08-model-and-id-subjects.rpl` | model/ID binding and adapter-specific methods |
| `09-method-only-adapters.rpl` | business RPCs without generated CRUD endpoints |

Model-level modes expose CRUD and unqualified methods. A method-level mode
restricts that operation to its adapter. If no model-level transport exists,
only explicitly annotated methods are generated.

HTTP paths are customizable with `NewModelHTTPHandlerAt` and
`NewModelHTTPClientAt`. NATS, Kafka, and WebSocket use small generated
interfaces so applications can choose their preferred client libraries.

Model-level routing defaults can also live in the schema:

```rpl
@transport(http,
    httpPath: "/api/users",
    brokerPrefix: "acme.users",
    kafkaGroup: "acme-users-rpc",
)
```

These values become the generated HTTP base path, NATS/Kafka subject or topic
prefix, and Kafka consumer group. Constructor parameters still allow an
application to override them at runtime.
