package plugins

import (
	"encoding/xml"
	"io"
	"os/exec"
)

type Manifest struct {
	XMLName     xml.Name `xml:"-"`
	Name        string   `xml:"name"`
	Author      string   `xml:"author,omitempty"`
	Version     string   `xml:"version"`
	DisplayName string   `xml:"displayName,omitempty"`
	Description string   `xml:"description,omitempty"`
	Executable  string   `xml:"executable,omitempty"`
	Entry       string   `xml:"entry,omitempty"`
	Path        string   `xml:"path,omitempty"`
	SDKVersion  string   `xml:"sdkVersion,omitempty"`
}

type Binary struct {
	Manifest     Manifest
	ManifestPath string
	ExecPath     string
}

type Process struct {
	Binary Binary
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
}

const (
	DefaultManifestName   = "manifest.xml"
	DefaultExecutableName = "attr"
	LegacyExecutableName  = "runtime"
)
