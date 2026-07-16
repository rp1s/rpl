package main

import (
	"os"
	"rpl/pkg/sdk"
)

func main() {
	attr := sdk.NewAttr("transport", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(transportModelSpec, transportFieldSpec)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeTransport)
	attr.HandleGenerateModel(func(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
		return generateTransport(req), nil
	})

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
