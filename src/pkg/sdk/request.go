package sdk

import (
	"fmt"
	"strings"
)

func DecodeGenerateRequest(msg Message) (GenerateRequest, error) {
	var req GenerateRequest
	if err := Decode(msg.Value, &req); err != nil {
		return GenerateRequest{}, err
	}

	return req, nil
}

func (request GenerateRequest) Validate() error {
	if strings.TrimSpace(request.Model.Name) == "" {
		return fmt.Errorf("model name is required")
	}
	if strings.TrimSpace(request.Runtime.Name) == "" {
		return fmt.Errorf("runtime name is required")
	}

	return nil
}
