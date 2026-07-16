package main

import (
	"fmt"
	"os"
	"rpl/pkg/sdk"
	"strings"
)

func main() {
	attr := sdk.NewAttr("mongodb", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(mongodbModelSpec, mongodbFieldSpec)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.Handle("query", func(msg sdk.Message) (any, error) {
		query, ok := msg.Value.(string)
		if !ok || strings.TrimSpace(query) == "" {
			return nil, fmt.Errorf("query must be string")
		}

		return map[string]any{
			"query": query,
			"ok":    true,
		}, nil
	})
	attr.HandleAnalyzeModel(analyzeMongoDB)
	attr.HandleGenerateModel(generateMongoDB)

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
