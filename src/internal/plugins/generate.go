package plugins

import (
	"encoding/json"
	"fmt"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"strings"
)

func GenerateModel(name string, author string, request sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	if err := request.Validate(); err != nil {
		return sdk.GenerateResponse{}, err
	}

	response, err := Request(name, author, map[string]any{
		"action": sdk.GenerateModelAction,
		"data":   request,
	})
	if err != nil {
		return sdk.GenerateResponse{}, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(response, &object); err == nil {
		if problem, ok := object["error"]; ok {
			var message string
			if err := json.Unmarshal(problem, &message); err == nil && strings.TrimSpace(message) != "" {
				return sdk.GenerateResponse{}, fmt.Errorf(
					localize.Text("attr %q автора %q вернул ошибку: %s", "attr %q by author %q returned an error: %s"),
					name,
					author,
					message,
				)
			}
		}
		if wrapped, ok := object["result"]; ok {
			var result sdk.GenerateResponse
			if err := json.Unmarshal(wrapped, &result); err != nil {
				return sdk.GenerateResponse{}, fmt.Errorf(
					localize.Text("разбор ответа генерации attr %q автора %q: %w", "decode generation response from attr %q by author %q: %w"),
					name,
					author,
					err,
				)
			}

			return result, nil
		}
	}

	var result sdk.GenerateResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return sdk.GenerateResponse{}, fmt.Errorf(
			localize.Text("разбор ответа генерации attr %q автора %q: %w", "decode generation response from attr %q by author %q: %w"),
			name,
			author,
			err,
		)
	}

	return result, nil
}
