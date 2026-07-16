package plugins

import (
	"encoding/json"
	"fmt"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"strings"
)

func AnalyzeModel(name string, author string, request sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	if err := request.Validate(); err != nil {
		return sdk.AnalyzeResponse{}, err
	}

	response, err := Request(name, author, map[string]any{
		"action": sdk.AnalyzeModelAction,
		"data":   request,
	})
	if err != nil {
		return sdk.AnalyzeResponse{}, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(response, &object); err == nil {
		if problem, ok := object["error"]; ok {
			var message string
			if err := json.Unmarshal(problem, &message); err == nil && strings.TrimSpace(message) != "" {
				return sdk.AnalyzeResponse{}, fmt.Errorf(
					localize.Text("attr %q автора %q вернул ошибку: %s", "attr %q by author %q returned an error: %s"),
					name,
					author,
					message,
				)
			}
		}
		if wrapped, ok := object["result"]; ok {
			var result sdk.AnalyzeResponse
			if err := json.Unmarshal(wrapped, &result); err != nil {
				return sdk.AnalyzeResponse{}, fmt.Errorf(
					localize.Text("разбор ответа анализа attr %q автора %q: %w", "decode analysis response from attr %q by author %q: %w"),
					name,
					author,
					err,
				)
			}

			return result, nil
		}
	}

	var result sdk.AnalyzeResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return sdk.AnalyzeResponse{}, fmt.Errorf(
			localize.Text("разбор ответа анализа attr %q автора %q: %w", "decode analysis response from attr %q by author %q: %w"),
			name,
			author,
			err,
		)
	}

	return result, nil
}
