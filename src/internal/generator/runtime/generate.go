package runtime

import (
	"rpl/internal/plugins"
	"rpl/pkg/sdk"
)

func GenerateModel(name string, author string, request sdk.GenerateRequest) (sdk.GenerateResponse, error) {
	return plugins.GenerateModel(name, author, request)
}
