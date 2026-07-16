package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type Message struct {
	Key     string
	Value   any
	Payload map[string]any
}

type HandlerFunc func(Message) (any, error)

type Server struct {
	Name     string
	Author   string
	stdin    *json.Decoder
	stdout   *json.Encoder
	handlers map[string]HandlerFunc
}

type Runtime = Server
type Plugin = Server

type GenerateModelHandler func(GenerateRequest) (GenerateResponse, error)
type AnalyzeModelHandler func(GenerateRequest) (AnalyzeResponse, error)
type GenerateFileHandler func(GenerateRequest) (GenerateResponse, error)
type AnalyzeFileHandler func(GenerateRequest) (AnalyzeResponse, error)
type DocsModelHandler func(DocsRequest) (DocsResponse, error)
type DocsFileHandler func(DocsRequest) (DocsResponse, error)

func New(name string, author string) *Server {
	return NewServer(name, author)
}

func NewPlugin(name string, author string) *Server {
	return NewServer(name, author)
}

func NewAttr(name string, author string) *Server {
	return NewServer(name, author)
}

func NewServer(name string, author string) *Server {
	return NewServerWithIO(name, author, os.Stdin, os.Stdout)
}

func NewWithIO(name string, author string, input io.Reader, output io.Writer) *Server {
	return NewServerWithIO(name, author, input, output)
}

func NewServerWithIO(name string, author string, input io.Reader, output io.Writer) *Server {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}

	return &Server{
		Name:     strings.TrimSpace(name),
		Author:   strings.TrimSpace(author),
		stdin:    json.NewDecoder(input),
		stdout:   json.NewEncoder(output),
		handlers: make(map[string]HandlerFunc),
	}
}

func (server *Server) Handle(key string, handler HandlerFunc) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" || handler == nil {
		return
	}

	server.handlers[trimmedKey] = handler
}

func (server *Server) HandlePing() {
	server.Handle("ping", func(msg Message) (any, error) {
		return map[string]any{
			"pong": msg.Value,
		}, nil
	})
}

func (server *Server) HandleGenerateModel(handler GenerateModelHandler) {
	if handler == nil {
		return
	}

	server.Handle(GenerateModelAction, func(msg Message) (any, error) {
		req, err := DecodeGenerateRequest(msg)
		if err != nil {
			return nil, err
		}

		return handler(req)
	})
}

func (server *Server) HandleGenerateFile(handler GenerateFileHandler) {
	if handler == nil {
		return
	}

	server.Handle(GenerateFileAction, func(msg Message) (any, error) {
		req, err := DecodeGenerateRequest(msg)
		if err != nil {
			return nil, err
		}

		return handler(req)
	})
}

func (server *Server) HandleAnalyzeModel(handler AnalyzeModelHandler) {
	if handler == nil {
		return
	}

	server.Handle(AnalyzeModelAction, func(msg Message) (any, error) {
		req, err := DecodeGenerateRequest(msg)
		if err != nil {
			return nil, err
		}

		return handler(req)
	})
}

func (server *Server) HandleAnalyzeFile(handler AnalyzeFileHandler) {
	if handler == nil {
		return
	}

	server.Handle(AnalyzeFileAction, func(msg Message) (any, error) {
		req, err := DecodeGenerateRequest(msg)
		if err != nil {
			return nil, err
		}

		return handler(req)
	})
}

func (server *Server) HandleDescribeAttrs(specs ...AttrSpec) {
	normalized := normalizeAttrSpecs(server.Name, specs)
	server.Handle(DescribeAttrsAction, func(msg Message) (any, error) {
		_ = msg
		return DescribeAttrsResponse{Specs: normalized}, nil
	})
}

func (server *Server) HandleDescribeCapabilities(capabilities AttrCapabilities) {
	server.Handle(DescribeCapabilitiesAction, func(msg Message) (any, error) {
		_ = msg
		return DescribeCapabilitiesResponse{
			Name:         server.Name,
			Author:       server.Author,
			Capabilities: capabilities,
		}, nil
	})
}

func (server *Server) HandleDocsModel(handler DocsModelHandler) {
	if handler == nil {
		return
	}
	server.Handle(DocsModelAction, func(msg Message) (any, error) {
		req, err := decodeDocsRequest(msg)
		if err != nil {
			return nil, err
		}
		return handler(req)
	})
}

func (server *Server) HandleDocsFile(handler DocsFileHandler) {
	if handler == nil {
		return
	}
	server.Handle(DocsFileAction, func(msg Message) (any, error) {
		req, err := decodeDocsRequest(msg)
		if err != nil {
			return nil, err
		}
		return handler(req)
	})
}

func (server *Server) Run() error {
	for {
		var payload map[string]any
		if err := server.stdin.Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("decode runtime request: %w", err)
		}

		response, err := server.dispatch(payload)
		if err != nil {
			if writeErr := server.write(map[string]any{
				"error": err.Error(),
			}); writeErr != nil {
				return writeErr
			}
			continue
		}

		if err := server.write(response); err != nil {
			return err
		}
	}
}

func (server *Server) dispatch(payload map[string]any) (any, error) {
	message, err := server.message(payload)
	if err != nil {
		return nil, err
	}

	handler, ok := server.handlers[message.Key]
	if !ok {
		return nil, fmt.Errorf("unknown action %q", message.Key)
	}

	return handler(message)
}

func (server *Server) message(payload map[string]any) (Message, error) {
	if len(payload) == 0 {
		return Message{}, errors.New("runtime request is empty")
	}

	if actionValue, ok := payload["action"]; ok {
		action, ok := actionValue.(string)
		if !ok || strings.TrimSpace(action) == "" {
			return Message{}, errors.New("runtime action must be string")
		}

		value, hasValue := payload["data"]
		if !hasValue {
			value = payload
		}

		return Message{
			Key:     strings.TrimSpace(action),
			Value:   value,
			Payload: payload,
		}, nil
	}

	for key, value := range payload {
		return Message{
			Key:     key,
			Value:   value,
			Payload: payload,
		}, nil
	}

	return Message{}, errors.New("runtime request is empty")
}

func (server *Server) write(response any) error {
	if object, ok := response.(map[string]any); ok {
		if _, exists := object["runtime"]; !exists {
			object["runtime"] = server.Name
		}
		if _, exists := object["author"]; !exists {
			object["author"] = server.Author
		}

		return server.stdout.Encode(object)
	}

	return server.stdout.Encode(map[string]any{
		"runtime": server.Name,
		"author":  server.Author,
		"result":  response,
	})
}

func normalizeAttrSpecs(defaultNamespace string, specs []AttrSpec) []AttrSpec {
	if len(specs) == 0 {
		if strings.TrimSpace(defaultNamespace) == "" {
			return nil
		}
		return []AttrSpec{{Namespace: strings.TrimSpace(defaultNamespace)}}
	}

	items := make([]AttrSpec, 0, len(specs))
	for _, item := range specs {
		if strings.TrimSpace(item.Namespace) == "" {
			item.Namespace = strings.TrimSpace(defaultNamespace)
		}
		if strings.TrimSpace(item.Namespace) == "" {
			continue
		}
		items = append(items, item)
	}

	return items
}

func Decode(value any, target any) error {
	if target == nil {
		return errors.New("decode target is nil")
	}

	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode message value: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode message value: %w", err)
	}

	return nil
}

func decodeDocsRequest(msg Message) (DocsRequest, error) {
	var req DocsRequest
	if err := Decode(msg.Value, &req); err != nil {
		return DocsRequest{}, err
	}
	return req, nil
}
