package runtime

import (
	"io"
	rootsdk "rpl/pkg/sdk"
)

const (
	AnalyzeModelAction         = rootsdk.AnalyzeModelAction
	AnalyzeFileAction          = rootsdk.AnalyzeFileAction
	GenerateModelAction        = rootsdk.GenerateModelAction
	GenerateFileAction         = rootsdk.GenerateFileAction
	DescribeAttrsAction        = rootsdk.DescribeAttrsAction
	DescribeCapabilitiesAction = rootsdk.DescribeCapabilitiesAction
	TypesCatalogAction         = rootsdk.TypesCatalogAction
	DocsModelAction            = rootsdk.DocsModelAction
	DocsFileAction             = rootsdk.DocsFileAction
)

type Message = rootsdk.Message
type HandlerFunc = rootsdk.HandlerFunc
type Server = rootsdk.Server
type Runtime = rootsdk.Runtime
type Plugin = rootsdk.Plugin

type GenerateModelHandler = rootsdk.GenerateModelHandler
type AnalyzeModelHandler = rootsdk.AnalyzeModelHandler
type GenerateFileHandler = rootsdk.GenerateFileHandler
type AnalyzeFileHandler = rootsdk.AnalyzeFileHandler
type DocsModelHandler = rootsdk.DocsModelHandler
type DocsFileHandler = rootsdk.DocsFileHandler

type AttrCapabilities = rootsdk.AttrCapabilities
type DescribeCapabilitiesResponse = rootsdk.DescribeCapabilitiesResponse

func New(name string, author string) *Server {
	return rootsdk.New(name, author)
}

func NewPlugin(name string, author string) *Server {
	return rootsdk.NewPlugin(name, author)
}

func NewAttr(name string, author string) *Server {
	return rootsdk.NewAttr(name, author)
}

func NewServer(name string, author string) *Server {
	return rootsdk.NewServer(name, author)
}

func NewWithIO(name string, author string, input io.Reader, output io.Writer) *Server {
	return rootsdk.NewWithIO(name, author, input, output)
}

func NewServerWithIO(name string, author string, input io.Reader, output io.Writer) *Server {
	return rootsdk.NewServerWithIO(name, author, input, output)
}

func Decode(value any, target any) error {
	return rootsdk.Decode(value, target)
}
