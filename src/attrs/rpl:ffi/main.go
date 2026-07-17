package main

import (
	"os"
	"rpl/pkg/sdk"
)

func main() {
	attr := sdk.NewAttr("ffi", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(ffiModelSpec, ffiMemberSpec)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeFFI)
	attr.HandleGenerateModel(generateFFI)

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
