package main

import (
	"os"
	"rpl/pkg/sdk"
)

func main() {
	attr := sdk.NewAttr("wasm", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(wasmModelSpec, wasmMemberSpec)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeWASM)
	attr.HandleGenerateModel(generateWASM)

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
