package main

import (
	"fmt"
	"rpl/pkg/sdk"
	"strings"
)

func generateValidate(req sdk.GenerateRequest) sdk.GenerateResponse {
	builder := sdk.NewCodeBuilder()
	modelImportAlias := "modelpkg"
	modelType := modelImportAlias + "." + req.Model.Name

	validationLines := make([]string, 0)
	hashLines := make([]string, 0)

	needsErrors := false
	needsFmt := false
	needsMail := false
	needsURL := false
	needsRegexp := false
	needsStrings := false
	needsTime := false
	needsSHA := false
	needsHex := false

	hashGuardName := sdk.LowerCamel(req.Model.Name) + "LooksHashed"
	phonePatternName := sdk.LowerCamel(req.Model.Name) + "PhonePattern"

	for _, field := range req.Model.ActiveFields("validate") {
		lines, flags := validationCodeForField(field, phonePatternName)
		validationLines = append(validationLines, lines...)
		needsErrors = needsErrors || flags["errors"]
		needsFmt = needsFmt || flags["fmt"]
		needsMail = needsMail || flags["net/mail"]
		needsURL = needsURL || flags["net/url"]
		needsRegexp = needsRegexp || flags["regexp"]
		needsStrings = needsStrings || flags["strings"]
		needsTime = needsTime || flags["time"]

		hashCode, hashFlags := hashCodeForField(field, hashGuardName)
		if hashCode != "" {
			hashLines = append(hashLines, hashCode)
		}
		needsSHA = needsSHA || hashFlags["crypto/sha256"]
		needsHex = needsHex || hashFlags["encoding/hex"]
	}

	if len(validationLines) == 0 && len(hashLines) == 0 {
		return builder.Response()
	}

	builder.AddImport(validationModelImportPath(req.File), modelImportAlias)
	if needsErrors {
		builder.AddImport("errors")
	}
	if needsFmt {
		builder.AddImport("fmt")
	}
	if needsMail {
		builder.AddImport("net/mail")
	}
	if needsURL {
		builder.AddImport("net/url")
	}
	if needsRegexp {
		builder.AddImport("regexp")
		builder.AddOrderedBlock("validate.phone.pattern", fmt.Sprintf("var %s = regexp.MustCompile(%q)", phonePatternName, `^\+?[0-9][0-9\s\-()]{4,}$`), 0)
	}
	if needsStrings {
		builder.AddImport("strings")
	}
	if needsTime {
		builder.AddImport("time")
	}
	if needsSHA {
		builder.AddImport("crypto/sha256")
	}
	if needsHex {
		builder.AddImport("encoding/hex")
	}

	if len(validationLines) > 0 {
		builder.AddOrderedBlock("validate.errors", generateValidationErrorsMethod(modelType, req.Model.Name, validationLines), 10)
		builder.AddOrderedBlock("validate.method", generateValidateMethod(modelType, req.Model.Name), 20)
	}
	if len(hashLines) > 0 {
		builder.AddOrderedBlock("validate.hash.check", generateHashGuardMethod(hashGuardName), 30)
		builder.AddOrderedBlock("validate.hash.method", generateHashMethod(modelType, req.Model.Name, hashLines), 40)
	}

	body, err := sdk.RenderGoFile("validation", builder.Response())
	if err != nil {
		return sdk.GenerateResponse{}
	}

	return sdk.GenerateResponse{
		Files: []sdk.GeneratedFile{{
			Path:    "validation/validation.gen.go",
			Content: string(body),
		}},
	}
}

func validationModelImportPath(file sdk.FileContext) string {
	if strings.TrimSpace(file.GoPackagePath) != "" {
		return strings.TrimSpace(file.GoPackagePath)
	}
	return ".."
}

func generateValidationErrorsMethod(modelType string, modelName string, lines []string) string {
	body := make([]string, 0, len(lines)+2)
	body = append(body, "errs := make([]error, 0)")
	body = append(body, lines...)
	body = append(body, "return errs")

	return sdk.WithDocComment(
		fmt.Sprintf("func Errors(model %s) []error {\n\t%s\n}", modelType, strings.Join(body, "\n\t")),
		"Errors собирает все ошибки валидации для модели %s.",
		"Errors collects every validation error for model %s.",
		modelName,
	)
}

func generateValidateMethod(modelType string, modelName string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func Validate(model %s) error {\n\terrs := Errors(model)\n\tif len(errs) == 0 {\n\t\treturn nil\n\t}\n\n\treturn errors.Join(errs...)\n}", modelType),
		"Validate возвращает объединенную ошибку валидации для модели %s.",
		"Validate returns the joined validation error for model %s.",
		modelName,
	)
}

func generateHashGuardMethod(fn string) string {
	return sdk.WithDocComment(
		fmt.Sprintf("func %s(value string) bool {\n\tif len(value) != 64 {\n\t\treturn false\n\t}\n\n\t_, err := hex.DecodeString(value)\n\treturn err == nil\n}", fn),
		"%s проверяет, похоже ли строковое значение на уже захешированное.",
		"%s checks whether the string already looks hashed.",
		fn,
	)
}

func generateHashMethod(modelType string, modelName string, lines []string) string {
	body := append([]string{}, lines...)
	return sdk.WithDocComment(
		fmt.Sprintf("func HashSensitiveFields(model *%s) {\n\t%s\n}", modelType, strings.Join(body, "\n\t")),
		"HashSensitiveFields хеширует чувствительные строковые поля модели %s перед сохранением.",
		"HashSensitiveFields hashes sensitive string fields on model %s before persistence.",
		modelName,
	)
}

func validationCodeForField(field sdk.Field, phonePatternName string) ([]string, map[string]bool) {
	lines := make([]string, 0)
	flags := map[string]bool{}
	fieldExpr := "model." + field.Name
	values := field.ResolvedValues("validate")
	if values["required"].BoolValue() {
		flags["fmt"] = true
		if field.Type.IsString() {
			flags["strings"] = true
		}
		lines = append(lines, requiredValidation(field, fieldExpr))
	}

	if min := strings.TrimSpace(values["min"].String()); min != "" {
		flags["fmt"] = true
		lines = append(lines, guardField(field, minValidation(field, fieldExpr, min)))
	}
	if max := strings.TrimSpace(values["max"].String()); max != "" {
		flags["fmt"] = true
		lines = append(lines, guardField(field, maxValidation(field, fieldExpr, max)))
	}
	if minLen := strings.TrimSpace(values["minLen"].String()); minLen != "" {
		flags["fmt"] = true
		lines = append(lines, guardField(field, fmt.Sprintf("if len(%s) < %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), minLen, field.Name+" length is too small")))
	}
	if maxLen := strings.TrimSpace(values["maxLen"].String()); maxLen != "" {
		flags["fmt"] = true
		lines = append(lines, guardField(field, fmt.Sprintf("if len(%s) > %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), maxLen, field.Name+" length is too large")))
	}
	if values["email"].BoolValue() {
		flags["fmt"] = true
		flags["net/mail"] = true
		lines = append(lines, guardField(field, emailValidation(field, fieldExpr)))
	}
	if values["url"].BoolValue() {
		flags["fmt"] = true
		flags["net/url"] = true
		lines = append(lines, guardField(field, urlValidation(field, fieldExpr)))
	}
	if values["phone"].BoolValue() {
		flags["fmt"] = true
		flags["regexp"] = true
		lines = append(lines, guardField(field, phoneValidation(field, fieldExpr, phonePatternName)))
	}
	if values["uuid"].BoolValue() {
		flags["fmt"] = true
		flags["regexp"] = true
		lines = append(lines, guardField(field, patternValidation(field, fieldExpr, `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`, "must be uuid")))
	}
	if pattern := strings.TrimSpace(values["pattern"].String()); pattern != "" {
		flags["fmt"] = true
		flags["regexp"] = true
		lines = append(lines, guardField(field, patternValidation(field, fieldExpr, pattern, "has invalid format")))
	}
	if values["past"].BoolValue() {
		flags["fmt"] = true
		flags["time"] = true
		lines = append(lines, guardField(field, fmt.Sprintf("if !%s.Before(time.Now()) {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), field.Name+" must be in the past")))
	}

	if len(lines) > 0 {
		flags["errors"] = true
	}

	return lines, flags
}

func requiredValidation(field sdk.Field, fieldExpr string) string {
	message := field.Name + " is required"
	switch {
	case field.Type.Optional && field.Type.IsString():
		return fmt.Sprintf("if %s == nil || strings.TrimSpace(*%s) == \"\" {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, fieldExpr, message)
	case field.Type.Optional && field.Type.IsTime():
		return fmt.Sprintf("if %s == nil || %s.IsZero() {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, fieldExpr, message)
	case field.Type.Optional:
		return fmt.Sprintf("if %s == nil {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, message)
	case field.Type.IsString():
		return fmt.Sprintf("if strings.TrimSpace(%s) == \"\" {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, message)
	case field.Type.IsList:
		return fmt.Sprintf("if len(%s) == 0 {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, message)
	case field.Type.IsTime():
		return fmt.Sprintf("if %s.IsZero() {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", fieldExpr, message)
	default:
		return ""
	}
}

func patternValidation(field sdk.Field, fieldExpr string, pattern string, message string) string {
	if field.Type.IsList {
		return fmt.Sprintf("for _, item := range %s {\n\t\tmatched, _ := regexp.MatchString(%q, item)\n\t\tif !matched {\n\t\t\terrs = append(errs, fmt.Errorf(%q))\n\t\t}\n\t}", fieldExpr, pattern, field.Name+" "+message)
	}
	return fmt.Sprintf("if matched, _ := regexp.MatchString(%q, %s); !matched {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", pattern, accessField(field, fieldExpr), field.Name+" "+message)
}

func hashCodeForField(field sdk.Field, hashGuardName string) (string, map[string]bool) {
	flags := map[string]bool{}
	shouldHash := strings.TrimSpace(field.ResolvedValues("validate")["hash"].String()) != ""
	if !shouldHash || !field.Type.IsString() {
		return "", flags
	}

	flags["crypto/sha256"] = true
	flags["encoding/hex"] = true
	fieldExpr := "model." + field.Name
	if field.Type.Optional {
		return fmt.Sprintf("if %s != nil && *%s != \"\" && !%s(*%s) {\n\t\tsum := sha256.Sum256([]byte(*%s))\n\t\tvalue := hex.EncodeToString(sum[:])\n\t\t%s = &value\n\t}", fieldExpr, fieldExpr, hashGuardName, fieldExpr, fieldExpr, fieldExpr), flags
	}

	return fmt.Sprintf("if %s != \"\" && !%s(%s) {\n\t\tsum := sha256.Sum256([]byte(%s))\n\t\t%s = hex.EncodeToString(sum[:])\n\t}", fieldExpr, hashGuardName, fieldExpr, fieldExpr, fieldExpr), flags
}

func minValidation(field sdk.Field, fieldExpr string, min string) string {
	switch {
	case field.Type.IsString():
		return fmt.Sprintf("if len(%s) < %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), min, field.Name+" is too short")
	case field.Type.IsNumeric():
		return fmt.Sprintf("if %s < %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), min, field.Name+" is too small")
	default:
		return ""
	}
}

func maxValidation(field sdk.Field, fieldExpr string, max string) string {
	switch {
	case field.Type.IsString():
		return fmt.Sprintf("if len(%s) > %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), max, field.Name+" is too long")
	case field.Type.IsNumeric():
		return fmt.Sprintf("if %s > %s {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), max, field.Name+" is too large")
	default:
		return ""
	}
}

func emailValidation(field sdk.Field, fieldExpr string) string {
	if field.Type.IsList {
		return fmt.Sprintf("for _, item := range %s {\n\t\tif _, err := mail.ParseAddress(item); err != nil {\n\t\t\terrs = append(errs, fmt.Errorf(%q))\n\t\t}\n\t}", fieldExpr, field.Name+" must be email")
	}

	return fmt.Sprintf("if _, err := mail.ParseAddress(%s); err != nil {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), field.Name+" must be email")
}

func urlValidation(field sdk.Field, fieldExpr string) string {
	if field.Type.IsList {
		return fmt.Sprintf("for _, item := range %s {\n\t\tif _, err := url.ParseRequestURI(item); err != nil {\n\t\t\terrs = append(errs, fmt.Errorf(%q))\n\t\t}\n\t}", fieldExpr, field.Name+" must be url")
	}

	return fmt.Sprintf("if _, err := url.ParseRequestURI(%s); err != nil {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", accessField(field, fieldExpr), field.Name+" must be url")
}

func phoneValidation(field sdk.Field, fieldExpr string, phonePatternName string) string {
	if field.Type.IsList {
		return fmt.Sprintf("for _, item := range %s {\n\t\tif !%s.MatchString(item) {\n\t\t\terrs = append(errs, fmt.Errorf(%q))\n\t\t}\n\t}", fieldExpr, phonePatternName, field.Name+" must be phone")
	}

	return fmt.Sprintf("if !%s.MatchString(%s) {\n\t\terrs = append(errs, fmt.Errorf(%q))\n\t}", phonePatternName, accessField(field, fieldExpr), field.Name+" must be phone")
}

func guardField(field sdk.Field, code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	if field.Type.IsList || !field.Type.Optional {
		return code
	}

	return fmt.Sprintf("if model.%s != nil {\n\t\t%s\n\t}", field.Name, sdk.Indent(code, "\t\t"))
}

func accessField(field sdk.Field, fieldExpr string) string {
	if field.Type.IsList || !field.Type.Optional {
		return fieldExpr
	}

	return "(*" + fieldExpr + ")"
}
