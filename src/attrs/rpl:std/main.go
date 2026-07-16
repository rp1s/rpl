package main

import (
	"os"
	"rpl/pkg/sdk"
)

func main() {
	attr := sdk.NewAttr("std", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(stdAttrSpecs...)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeStd)
	attr.HandleGenerateModel(generateStd)

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
