package sdk

import (
	"fmt"
	"io"
	"strings"
)

type DiagnosticError struct {
	Message string
	Hint    string
	Detail  string
	Cause   error
}

func NewError(text string) *DiagnosticError {
	return &DiagnosticError{Message: strings.TrimSpace(text)}
}

func NewErrorf(format string, args ...any) *DiagnosticError {
	return NewError(fmt.Sprintf(format, args...))
}

func (err *DiagnosticError) Error() string {
	if err == nil {
		return ""
	}
	if strings.TrimSpace(err.Message) == "" {
		if err.Cause != nil {
			return err.Cause.Error()
		}
		return "unknown error"
	}
	if err.Cause != nil {
		return err.Message + ": " + err.Cause.Error()
	}
	return err.Message
}

func (err *DiagnosticError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (err *DiagnosticError) WithHint(text string) *DiagnosticError {
	if err == nil {
		return nil
	}
	err.Hint = strings.TrimSpace(text)
	return err
}

func (err *DiagnosticError) WithDetail(text string) *DiagnosticError {
	if err == nil {
		return nil
	}
	err.Detail = strings.TrimSpace(text)
	return err
}

func (err *DiagnosticError) WithCause(cause error) *DiagnosticError {
	if err == nil {
		return nil
	}
	err.Cause = cause
	return err
}

func PrintError(writer io.Writer, err error) {
	if writer == nil || err == nil {
		return
	}

	diagnostic, ok := err.(*DiagnosticError)
	if !ok {
		_, _ = fmt.Fprintln(writer, err.Error())
		return
	}

	if message := strings.TrimSpace(diagnostic.Message); message != "" {
		_, _ = fmt.Fprintln(writer, message)
	} else if diagnostic.Cause != nil {
		_, _ = fmt.Fprintln(writer, diagnostic.Cause.Error())
	}
	if detail := strings.TrimSpace(diagnostic.Detail); detail != "" {
		_, _ = fmt.Fprintln(writer, detail)
	}
	if hint := strings.TrimSpace(diagnostic.Hint); hint != "" {
		_, _ = fmt.Fprintln(writer, "hint: "+hint)
	}
	if diagnostic.Cause != nil && strings.TrimSpace(diagnostic.Detail) == "" {
		_, _ = fmt.Fprintln(writer, "cause: "+diagnostic.Cause.Error())
	}
}

func Text(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
