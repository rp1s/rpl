package error

import (
	"fmt"
	"io"
	"os"
	"rpl/pkg/ansi"
	"rpl/pkg/error/localize"
	"strings"
)

type Error struct {
	Kind    string
	Message string
	Hint    string
	Details []string

	FilePath string
	Source   string
	Line     int
	Column   int

	Cause error
}

func New(message string) *Error {
	return &Error{
		Message: strings.TrimSpace(message),
	}
}

func Newf(format string, args ...any) *Error {
	return New(fmt.Sprintf(format, args...))
}

func Wrap(err error, message string) *Error {
	item := New(message)
	item.Cause = err
	return item
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}

	switch {
	case strings.TrimSpace(err.Message) != "":
		return strings.TrimSpace(err.Message)
	case err.Cause != nil:
		return err.Cause.Error()
	default:
		return localize.Text("неизвестная ошибка", "unknown error")
	}
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}

	return err.Cause
}

func (err *Error) WithKind(kind string) *Error {
	if err == nil {
		return nil
	}

	err.Kind = strings.TrimSpace(kind)
	return err
}

func (err *Error) WithHint(hint string) *Error {
	if err == nil {
		return nil
	}

	err.Hint = strings.TrimSpace(hint)
	return err
}

func (err *Error) WithDetail(detail string) *Error {
	if err == nil {
		return nil
	}

	detail = strings.TrimSpace(detail)
	if detail == "" {
		return err
	}

	err.Details = append(err.Details, detail)
	return err
}

func (err *Error) WithLocation(filePath string, line int, column int) *Error {
	if err == nil {
		return nil
	}

	err.FilePath = strings.TrimSpace(filePath)
	err.Line = line
	err.Column = column
	return err
}

func (err *Error) WithSource(source string) *Error {
	if err == nil {
		return nil
	}

	err.Source = source
	return err
}

func (err *Error) Clone() *Error {
	if err == nil {
		return nil
	}

	cloned := *err
	cloned.Details = append([]string(nil), err.Details...)
	return &cloned
}

func Print(writer io.Writer, err error) {
	if writer == nil {
		writer = os.Stderr
	}
	if err == nil {
		return
	}

	_, _ = fmt.Fprintln(writer, Format(writer, err))
}

func Format(writer io.Writer, err error) string {
	if err == nil {
		return ""
	}

	items := flatten(err)
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return formatOne(writer, explain(items[0]))
	}

	lines := []string{
		ansi.Sprintln(writer, ansi.Error, fmt.Sprintf(localize.Text("Найдено %d проблем", "Found %d problems"), len(items))),
	}

	for i, item := range items {
		lines = append(lines, "")
		lines = append(lines, ansi.Heading(writer, fmt.Sprintf(localize.Text("Проблема %d", "Problem %d"), i+1)))
		lines = append(lines, formatOne(writer, explain(item)))
	}

	return strings.Join(lines, "\n")
}

func formatOne(writer io.Writer, item *Error) string {
	if item == nil {
		return ansi.Sprintln(writer, ansi.Error, localize.Text("Неизвестная ошибка", "Unknown error"))
	}

	lines := []string{
		ansi.Sprintln(writer, ansi.Error, item.Error()),
	}

	if location := item.locationLabel(); location != "" {
		lines = append(lines, formatMetaLine(writer, localize.Text("позиция:", "location:"), location))
	}

	if frame := item.codeFrame(writer); frame != "" {
		lines = append(lines, "")
		lines = append(lines, frame)
	}

	for _, detail := range item.Details {
		lines = append(lines, formatMetaLine(writer, localize.Text("деталь:", "detail:"), detail))
	}

	if hint := item.resolvedHint(); hint != "" {
		lines = append(lines, formatMetaLine(writer, localize.Text("подсказка:", "hint:"), hint))
	}

	if item.Cause != nil {
		cause := strings.TrimSpace(item.Cause.Error())
		if cause != "" && cause != item.Error() {
			lines = append(lines, formatMetaLine(writer, localize.Text("причина:", "cause:"), cause))
		}
	}

	return strings.Join(lines, "\n")
}

func explain(err error) *Error {
	if err == nil {
		return nil
	}

	if typed, ok := err.(*Error); ok {
		item := typed.Clone()
		if strings.TrimSpace(item.Hint) == "" {
			item.Hint = guessHint(item.Error())
		}
		return item
	}

	return New(err.Error()).WithHint(guessHint(err.Error()))
}

func flatten(err error) []error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface {
		Unwrap() []error
	}

	if multi, ok := err.(multiUnwrapper); ok {
		items := make([]error, 0)
		for _, item := range multi.Unwrap() {
			items = append(items, flatten(item)...)
		}
		return items
	}

	return []error{err}
}

func (err *Error) locationLabel() string {
	if err == nil {
		return ""
	}

	location := strings.TrimSpace(err.FilePath)
	if err.Line > 0 {
		location += fmt.Sprintf(":%d", err.Line)
		if err.Column > 0 {
			location += fmt.Sprintf(":%d", err.Column)
		}
	}

	return strings.TrimSpace(strings.TrimPrefix(location, ":"))
}

// codeFrame renders a small snippet around the failing line.
// It prefers the in-memory source captured during parsing and falls back to
// reading the file from disk when only a file path is available.
func (err *Error) codeFrame(writer io.Writer) string {
	if err == nil || err.Line <= 0 {
		return ""
	}

	source := err.Source
	if strings.TrimSpace(source) == "" && strings.TrimSpace(err.FilePath) != "" {
		body, readErr := os.ReadFile(err.FilePath)
		if readErr == nil {
			source = string(body)
		}
	}
	if strings.TrimSpace(source) == "" {
		return ""
	}

	lines := strings.Split(source, "\n")
	lineIndex := err.Line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		return ""
	}

	start := lineIndex - 1
	if start < 0 {
		start = 0
	}
	end := lineIndex + 1
	if end >= len(lines) {
		end = len(lines) - 1
	}

	width := len(fmt.Sprintf("%d", end+1))
	rendered := make([]string, 0, (end-start+1)+1)
	for i := start; i <= end; i++ {
		prefix := fmt.Sprintf("%*d | ", width, i+1)
		rendered = append(rendered, "  "+ansi.Muted(writer, prefix)+lines[i])
		if i != lineIndex {
			continue
		}

		caretColumn := err.Column
		if caretColumn <= 0 {
			caretColumn = 1
		}
		indicator := strings.Repeat(" ", max(width, 1)) + " | " + caretPadding(lines[i], caretColumn) + "^"
		rendered = append(rendered, "  "+ansi.Accent(writer, indicator))
	}

	return strings.Join(rendered, "\n")
}

func formatMetaLine(writer io.Writer, label string, value string) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if label == "" {
		return "  " + value
	}

	return "  " + ansi.Accent(writer, label) + " " + value
}

func (err *Error) resolvedHint() string {
	if err == nil {
		return ""
	}
	if strings.TrimSpace(err.Hint) != "" {
		return strings.TrimSpace(err.Hint)
	}

	return guessHint(err.Error())
}

func guessHint(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "unknown command"), strings.Contains(lower, "неизвестная команда"):
		return localize.Text("Запустите `rpl help`, чтобы увидеть доступные команды.", "Run `rpl help` to see the available commands.")
	case strings.Contains(lower, "unknown attr command"), strings.Contains(lower, "неизвестная attr-команда"):
		return localize.Text("Посмотрите `rpl attr list`, `rpl attr info author:name` или `rpl attr init author:name`.", "Try `rpl attr list`, `rpl attr info author:name`, or `rpl attr init author:name`.")
	case strings.Contains(lower, "attr"), strings.Contains(lower, "атриб"), strings.Contains(lower, "плагин"), strings.Contains(lower, "plugin"):
		if strings.Contains(lower, "not found") || strings.Contains(lower, "не найден") {
			return localize.Text("Проверьте `rpl attr list` или создайте каркас через `rpl attr init author:name`.", "Check `rpl attr list` or create a scaffold with `rpl attr init author:name`.")
		}
	case strings.Contains(lower, "recursive import"), strings.Contains(lower, "рекурсивный импорт"):
		return localize.Text("Вынесите общие модели в третий файл и оставьте импорты только в одну сторону.", "Move shared models into a third file and keep imports one-way.")
	case strings.Contains(lower, "unsupported target language"), strings.Contains(lower, "неподдерживаемый target language"):
		return localize.Text("Сейчас проект умеет рендерить только `golang`.", "Right now the project only supports the `golang` target.")
	case strings.Contains(lower, "path to .rpl file is required"), strings.Contains(lower, "не указан путь до файла .rpl"):
		return localize.Text("Пример: `rpl run src/main.rpl`.", "Example: `rpl run src/main.rpl`.")
	case strings.Contains(lower, "duplicate model name"), strings.Contains(lower, "дублирующееся имя модели"):
		return localize.Text("Переименуйте одну из моделей или вынесите общую модель в отдельный импортируемый файл.", "Rename one of the models or move the shared model into a separate imported file.")
	case strings.Contains(lower, "duplicate import alias"), strings.Contains(lower, "дублирующийся алиас импорта"):
		return localize.Text("Поменяйте алиас одного из импортов, чтобы они не совпадали.", "Give one of the imports a different alias.")
	}

	return ""
}

func caretPadding(line string, column int) string {
	if column <= 1 {
		return ""
	}

	runes := []rune(line)
	if column-1 > len(runes) {
		column = len(runes) + 1
	}

	padding := make([]rune, 0, column-1)
	for _, item := range runes[:column-1] {
		if item == '\t' {
			padding = append(padding, '\t')
			continue
		}
		padding = append(padding, ' ')
	}

	return string(padding)
}

func max(a int, b int) int {
	if a > b {
		return a
	}

	return b
}
