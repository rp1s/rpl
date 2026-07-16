package generator

import (
	"errors"
	"fmt"
	targetpkg "rpl/internal/generator/target"
	"rpl/internal/plugins"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"sort"
	"strings"
)

type claimOwner struct {
	runtime sdk.RuntimeRef
	model   string
}

func (gen *Generator) analyzeAttrs(renderer targetpkg.Renderer, targetLang string, outputDir string) error {
	if gen == nil || gen.File == nil || renderer == nil {
		return nil
	}

	imports := gen.fileImports()
	runtimes := gen.runtimeRefs()
	if err := gen.ensureRuntimeNamespaces(runtimes); err != nil {
		return err
	}
	modelNames := gen.modelNames()
	claims := make(map[string]claimOwner)
	problems := make([]error, 0)

	for _, model := range gen.File.Models() {
		if model == nil || strings.TrimSpace(model.GeneratedFrom) != "" {
			continue
		}

		layout := gen.resolveModelLayout(renderer, outputDir, model)
		for _, runtimeRef := range runtimes {
			request, ok := gen.generateRequest(
				model,
				imports,
				runtimes,
				modelNames,
				gen.runtimeModelNames(runtimeRef),
				runtimeRef,
				targetLang,
				layout.ModelDir,
				layout.ModelPackage,
				generatedFileStem(layout.ModelDirName),
			)
			if !ok {
				continue
			}

			if err := plugins.EnsureAvailableAt(gen.SourcePath, runtimeRef.Name, runtimeRef.Author); err != nil {
				problems = append(problems, err)
				continue
			}

			response, err := plugins.AnalyzeModel(runtimeRef.Name, runtimeRef.Author, request)
			if err != nil {
				problems = append(problems, err)
				continue
			}

			for _, item := range response.Diagnostics {
				problems = append(problems, diagnosticToError(item))
			}
			for _, claim := range response.Claims {
				if err := registerClaim(claims, claim, runtimeRef, model.Name); err != nil {
					problems = append(problems, err)
				}
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}

	return errors.Join(problems...)
}

func diagnosticToError(item sdk.Diagnostic) error {
	problem := rplerr.New(strings.TrimSpace(item.Message))
	if strings.TrimSpace(item.Hint) != "" {
		problem.WithHint(item.Hint)
	}
	if strings.TrimSpace(item.Detail) != "" {
		problem.WithDetail(item.Detail)
	}
	if strings.TrimSpace(item.Path) != "" || item.Line > 0 || item.Column > 0 {
		problem.WithLocation(item.Path, item.Line, item.Column)
	}

	return problem
}

func registerClaim(claims map[string]claimOwner, claim sdk.Claim, runtime sdk.RuntimeRef, modelName string) error {
	kind := strings.TrimSpace(claim.Kind)
	name := strings.TrimSpace(claim.Name)
	scope := strings.TrimSpace(claim.Scope)
	if kind == "" || name == "" {
		return nil
	}

	key := strings.Join([]string{kind, name, scope}, "|")
	owner, exists := claims[key]
	if !exists {
		claims[key] = claimOwner{runtime: runtime, model: modelName}
		return nil
	}
	if owner.runtime == runtime && owner.model == modelName {
		return nil
	}

	switch kind {
	case "identifier":
		return rplerr.Newf(
			localize.Text("top-level helper name %q collides", "top-level helper name %q collides"),
			name,
		).WithDetail(fmt.Sprintf(localize.Text(
			"Конфликт между attr %q модели %q и attr %q модели %q.",
			"Collision between attr %q on model %q and attr %q on model %q.",
		), owner.runtime.Name, owner.model, runtime.Name, modelName))
	case "file":
		return rplerr.Newf(
			localize.Text("collision in generated file %q", "collision in generated file %q"),
			name,
		).WithDetail(fmt.Sprintf(localize.Text(
			"Конфликт между attr %q модели %q и attr %q модели %q.",
			"Collision between attr %q on model %q and attr %q on model %q.",
		), owner.runtime.Name, owner.model, runtime.Name, modelName))
	case "field.domain":
		fieldName := scope
		if strings.Contains(scope, ".") {
			parts := strings.Split(scope, ".")
			fieldName = parts[len(parts)-1]
		}
		detailOwners := []string{
			owner.runtime.Name,
			runtime.Name,
		}
		sort.Strings(detailOwners)
		return rplerr.Newf(
			localize.Text("поле %q имеет конфликтующие attrs в домене %q", "field %q has conflicting attrs in domain %q"),
			fieldName,
			name,
		).WithDetail(fmt.Sprintf(localize.Text(
			"Поле %s одновременно захвачено attrs: %s.",
			"Field %s is claimed by attrs: %s.",
		), scope, strings.Join(detailOwners, ", ")))
	default:
		return rplerr.Newf(
			localize.Text("analysis claim %q for %q collides", "analysis claim %q for %q collides"),
			kind,
			name,
		)
	}
}
