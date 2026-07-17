package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateNATSTransportFile(plan *transportPlan, mode transportModePlan) sdk.GeneratedFile {
	builder := sdk.NewCodeBuilder()
	for _, path := range []string{"context", "encoding/json", "errors", "fmt", "io", "strings"} {
		builder.AddImport(path)
	}
	builder.AddImport(plan.ModelImportPath, "modelpkg")
	builder.AddOrderedBlock("transport.nats", renderNATSTransport(plan, mode), 10)
	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil {
		return sdk.GeneratedFile{}
	}
	return sdk.GeneratedFile{Path: "transport/nats.gen.go", Content: string(body)}
}

func renderNATSTransport(plan *transportPlan, mode transportModePlan) string {
	brokerName := plan.Model.Name + "NATSBroker"
	serverName := plan.Model.Name + "NATSServer"
	clientName := plan.Model.Name + "NATSClient"
	defaultPrefix := plan.BrokerPrefix
	methodNames := quotedTransportMethodNames(mode.Methods)
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`// %s is the small request/reply surface required from a NATS adapter.
// Implement it with the NATS client used by the application.
type %s interface {
	Request(ctx context.Context, subject string, payload []byte) ([]byte, error)
	Subscribe(subject string, handler func(context.Context, []byte) ([]byte, error)) (io.Closer, error)
}

type %s struct {
	broker %s
	prefix string
	service %s
	subscriptions []io.Closer
}

func Start%s(broker %s, prefix string, service %s) (*%s, error) {
	if broker == nil || service == nil {
		return nil, fmt.Errorf("transport NATS broker and service are required")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), ".")
	if prefix == "" {
		prefix = %q
	}
	server := &%s{broker: broker, prefix: prefix, service: service}
	for _, method := range []string{%s} {
		method := method
		subscription, err := broker.Subscribe(server.subject(method), func(ctx context.Context, payload []byte) ([]byte, error) {
			response, dispatchErr := %s(service, ctx, %q, %s{Method: method, Payload: payload})
			if dispatchErr != nil {
				response = %s{Error: dispatchErr.Error()}
			}
			return json.Marshal(response)
		})
		if err != nil {
			_ = server.Close()
			return nil, err
		}
		server.subscriptions = append(server.subscriptions, subscription)
	}
	return server, nil
}

func (server *%s) subject(method string) string {
	return server.prefix + "." + strings.ToLower(method)
}

func (server *%s) Close() error {
	if server == nil {
		return nil
	}
	errs := make([]error, 0)
	for _, subscription := range server.subscriptions {
		if subscription != nil {
			if err := subscription.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	server.subscriptions = nil
	return errors.Join(errs...)
}

type %s struct {
	broker %s
	prefix string
}

func New%s(broker %s, prefix string) (*%s, error) {
	if broker == nil {
		return nil, fmt.Errorf("transport NATS broker is required")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), ".")
	if prefix == "" {
		prefix = %q
	}
	return &%s{broker: broker, prefix: prefix}, nil
}

func (client *%s) roundTrip(ctx context.Context, method string, requestValue any, responseValue any) error {
	if client == nil || client.broker == nil {
		return fmt.Errorf("transport NATS client is nil")
	}
	payload, err := json.Marshal(requestValue)
	if err != nil {
		return err
	}
	body, err := client.broker.Request(ctx, client.prefix+"."+strings.ToLower(method), payload)
	if err != nil {
		return err
	}
	var envelope %s
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if strings.TrimSpace(envelope.Error) != "" {
		return fmt.Errorf("transport NATS %%s: %%s", method, envelope.Error)
	}
	if responseValue == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, responseValue)
}`,
		brokerName, brokerName,
		serverName, brokerName, plan.ServiceName,
		serverName, brokerName, plan.ServiceName, serverName,
		defaultPrefix, serverName, methodNames,
		transportDispatchName(plan), transportModeNATS, plan.EnvelopeName, plan.ResponseName,
		serverName, serverName,
		clientName, brokerName, clientName, brokerName, clientName,
		defaultPrefix, clientName, clientName, plan.ResponseName,
	))
	if len(mode.Methods) > 0 {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportTypedClientMethods(plan, mode.Methods, clientName, "roundTrip"))
	}
	return builder.String()
}

func generateKafkaTransportFile(plan *transportPlan, mode transportModePlan) sdk.GeneratedFile {
	builder := sdk.NewCodeBuilder()
	for _, path := range []string{"context", "encoding/json", "errors", "fmt", "io", "strings"} {
		builder.AddImport(path)
	}
	builder.AddImport(plan.ModelImportPath, "modelpkg")
	builder.AddOrderedBlock("transport.kafka", renderKafkaTransport(plan, mode), 10)
	body, err := sdk.RenderGoFile("transport", builder.Response())
	if err != nil {
		return sdk.GeneratedFile{}
	}
	return sdk.GeneratedFile{Path: "transport/kafka.gen.go", Content: string(body)}
}

func renderKafkaTransport(plan *transportPlan, mode transportModePlan) string {
	brokerName := plan.Model.Name + "KafkaBroker"
	serverName := plan.Model.Name + "KafkaServer"
	clientName := plan.Model.Name + "KafkaClient"
	defaultPrefix := plan.BrokerPrefix
	defaultGroup := plan.KafkaGroup
	methodNames := quotedTransportMethodNames(mode.Methods)
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`// %s owns Kafka correlation IDs, reply topics, and consumer lifecycle.
// Generated code only requires this transport-neutral RPC surface.
type %s interface {
	Request(ctx context.Context, topic string, key []byte, payload []byte) ([]byte, error)
	Consume(topic string, group string, handler func(context.Context, []byte, []byte) ([]byte, error)) (io.Closer, error)
}

type %s struct {
	broker %s
	prefix string
	group string
	service %s
	consumers []io.Closer
}

func Start%s(broker %s, prefix string, group string, service %s) (*%s, error) {
	if broker == nil || service == nil {
		return nil, fmt.Errorf("transport Kafka broker and service are required")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), ".")
	if prefix == "" { prefix = %q }
	group = strings.TrimSpace(group)
	if group == "" { group = %q }
	server := &%s{broker: broker, prefix: prefix, group: group, service: service}
	for _, method := range []string{%s} {
		method := method
		consumer, err := broker.Consume(server.topic(method), group, func(ctx context.Context, key []byte, payload []byte) ([]byte, error) {
			_ = key
			response, dispatchErr := %s(service, ctx, %q, %s{Method: method, Payload: payload})
			if dispatchErr != nil { response = %s{Error: dispatchErr.Error()} }
			return json.Marshal(response)
		})
		if err != nil {
			_ = server.Close()
			return nil, err
		}
		server.consumers = append(server.consumers, consumer)
	}
	return server, nil
}

func (server *%s) topic(method string) string {
	return server.prefix + "." + strings.ToLower(method)
}

func (server *%s) Close() error {
	if server == nil { return nil }
	errs := make([]error, 0)
	for _, consumer := range server.consumers {
		if consumer != nil {
			if err := consumer.Close(); err != nil { errs = append(errs, err) }
		}
	}
	server.consumers = nil
	return errors.Join(errs...)
}

type %s struct {
	broker %s
	prefix string
	key func(method string, request any) []byte
}

func New%s(broker %s, prefix string, key func(string, any) []byte) (*%s, error) {
	if broker == nil { return nil, fmt.Errorf("transport Kafka broker is required") }
	prefix = strings.Trim(strings.TrimSpace(prefix), ".")
	if prefix == "" { prefix = %q }
	return &%s{broker: broker, prefix: prefix, key: key}, nil
}

func (client *%s) roundTrip(ctx context.Context, method string, requestValue any, responseValue any) error {
	if client == nil || client.broker == nil { return fmt.Errorf("transport Kafka client is nil") }
	payload, err := json.Marshal(requestValue)
	if err != nil { return err }
	var key []byte
	if client.key != nil { key = client.key(method, requestValue) }
	body, err := client.broker.Request(ctx, client.prefix+"."+strings.ToLower(method), key, payload)
	if err != nil { return err }
	var envelope %s
	if err := json.Unmarshal(body, &envelope); err != nil { return err }
	if strings.TrimSpace(envelope.Error) != "" { return fmt.Errorf("transport Kafka %%s: %%s", method, envelope.Error) }
	if responseValue == nil || len(envelope.Result) == 0 { return nil }
	return json.Unmarshal(envelope.Result, responseValue)
}`,
		brokerName, brokerName,
		serverName, brokerName, plan.ServiceName,
		serverName, brokerName, plan.ServiceName, serverName,
		defaultPrefix, defaultGroup, serverName, methodNames,
		transportDispatchName(plan), transportModeKafka, plan.EnvelopeName, plan.ResponseName,
		serverName, serverName,
		clientName, brokerName, clientName, brokerName, clientName,
		defaultPrefix, clientName, clientName, plan.ResponseName,
	))
	if len(mode.Methods) > 0 {
		builder.WriteString("\n\n")
		builder.WriteString(renderTransportTypedClientMethods(plan, mode.Methods, clientName, "roundTrip"))
	}
	return builder.String()
}

func quotedTransportMethodNames(methods []transportMethodPlan) string {
	items := make([]string, 0, len(methods))
	for _, method := range methods {
		items = append(items, fmt.Sprintf("%q", method.Name))
	}
	return strings.Join(items, ", ")
}
