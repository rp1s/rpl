package main

import (
	"os"
	"rpl/pkg/sdk"
)

func main() {
	attr := sdk.NewAttr("redis", "rpl")
	attr.HandlePing()
	attr.HandleDescribeAttrs(redisModelSpec, redisFieldSpec)
	attr.HandleDescribeCapabilities(sdk.AttrCapabilities{
		AnalyzeModel:  true,
		GenerateModel: true,
		DescribeAttrs: true,
	})
	attr.HandleAnalyzeModel(analyzeRedis)
	attr.HandleGenerateModel(func(req sdk.GenerateRequest) (sdk.GenerateResponse, error) {
		return generateRedis(req), nil
	})

	if err := attr.Run(); err != nil {
		sdk.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
