package plugins

import (
	"encoding/json"
	"fmt"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"strings"
)

func DescribeAttrs(name string, author string) (sdk.DescribeAttrsResponse, error) {
	return DescribeAttrsAt("", name, author)
}

func DescribeAttrsAt(basePath string, name string, author string) (sdk.DescribeAttrsResponse, error) {
	response, err := RequestAt(basePath, name, author, map[string]any{
		"action": sdk.DescribeAttrsAction,
	})
	if err != nil {
		return sdk.DescribeAttrsResponse{}, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(response, &object); err == nil {
		if problem, ok := object["error"]; ok {
			var message string
			if err := json.Unmarshal(problem, &message); err == nil && strings.TrimSpace(message) != "" {
				return sdk.DescribeAttrsResponse{}, fmt.Errorf(
					localize.Text("attr %q автора %q вернул ошибку: %s", "attr %q by author %q returned an error: %s"),
					name,
					author,
					message,
				)
			}
		}
		if wrapped, ok := object["result"]; ok {
			var result sdk.DescribeAttrsResponse
			if err := json.Unmarshal(wrapped, &result); err == nil {
				return result, nil
			}
		}
	}

	var result sdk.DescribeAttrsResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return sdk.DescribeAttrsResponse{}, fmt.Errorf(
			localize.Text("разбор ответа описания attr %q автора %q: %w", "decode describe response from attr %q by author %q: %w"),
			name,
			author,
			err,
		)
	}

	return result, nil
}

func DescribeCapabilities(name string, author string) (sdk.DescribeCapabilitiesResponse, error) {
	return DescribeCapabilitiesAt("", name, author)
}

func DescribeCapabilitiesAt(basePath string, name string, author string) (sdk.DescribeCapabilitiesResponse, error) {
	response, err := RequestAt(basePath, name, author, map[string]any{
		"action": sdk.DescribeCapabilitiesAction,
	})
	if err != nil {
		return sdk.DescribeCapabilitiesResponse{}, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(response, &object); err == nil {
		if problem, ok := object["error"]; ok {
			var message string
			if err := json.Unmarshal(problem, &message); err == nil && strings.TrimSpace(message) != "" {
				return sdk.DescribeCapabilitiesResponse{}, fmt.Errorf(
					localize.Text("attr %q автора %q вернул ошибку: %s", "attr %q by author %q returned an error: %s"),
					name,
					author,
					message,
				)
			}
		}
		if wrapped, ok := object["result"]; ok {
			var result sdk.DescribeCapabilitiesResponse
			if err := json.Unmarshal(wrapped, &result); err == nil {
				return result, nil
			}
		}
	}

	var result sdk.DescribeCapabilitiesResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return sdk.DescribeCapabilitiesResponse{}, fmt.Errorf(
			localize.Text("разбор ответа описания возможностей attr %q автора %q: %w", "decode capabilities response from attr %q by author %q: %w"),
			name,
			author,
			err,
		)
	}

	return result, nil
}
