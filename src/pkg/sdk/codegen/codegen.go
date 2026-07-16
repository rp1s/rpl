package codegen

import rootsdk "rpl/pkg/sdk"

type RuntimeRef = rootsdk.RuntimeRef
type FileContext = rootsdk.FileContext
type ImportRef = rootsdk.ImportRef
type Message = rootsdk.Message
type GenerateRequest = rootsdk.GenerateRequest
type CodeBlock = rootsdk.CodeBlock
type GeneratedFile = rootsdk.GeneratedFile
type GenerateResponse = rootsdk.GenerateResponse
type CodeBuilder = rootsdk.CodeBuilder

func DocComment(primary string, fallback string, args ...any) string {
	return rootsdk.DocComment(primary, fallback, args...)
}

func WithDocComment(code string, primary string, fallback string, args ...any) string {
	return rootsdk.WithDocComment(code, primary, fallback, args...)
}

func DecodeGenerateRequest(msg rootsdk.Message) (GenerateRequest, error) {
	return rootsdk.DecodeGenerateRequest(msg)
}

func NewCodeBuilder() *CodeBuilder {
	return rootsdk.NewCodeBuilder()
}

func RenderGoFile(packageName string, response GenerateResponse) ([]byte, error) {
	return rootsdk.RenderGoFile(packageName, response)
}
