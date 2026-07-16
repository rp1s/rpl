package jsonapi

import (
	"errors"
	"fmt"
	"rpl/internal/config"
	"rpl/internal/plugins"
	compilersvc "rpl/internal/service/compiler"
	languagesvc "rpl/internal/service/language"
	typecatalogsvc "rpl/internal/service/typecatalog"
	rplerr "rpl/pkg/error"
	"rpl/pkg/error/localize"
	"rpl/pkg/sdk"
	"sort"
	"strings"
)

type Server struct {
	server   *sdk.Server
	compiler *compilersvc.Service
	language *languagesvc.Service
	types    *typecatalogsvc.Service
}

type runRequest struct {
	Code string `json:"code"`
}

type checkRequest struct {
	Code string `json:"code"`
	Path string `json:"path,omitempty"`
}

type formatRequest struct {
	Code string `json:"code"`
	Path string `json:"path,omitempty"`
}

type autoImportRequest struct {
	Code string `json:"code"`
	Path string `json:"path,omitempty"`
}

type langRequest struct {
	Lang string `json:"lang"`
	Path string `json:"path,omitempty"`
}

type attrLookupRequest struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Path   string `json:"path,omitempty"`
}

type attrSearchRequest struct {
	Value string `json:"value"`
	Path  string `json:"path,omitempty"`
}

type typeCatalogRequest struct {
	Lang string `json:"lang,omitempty"`
	Path string `json:"path,omitempty"`
}

type attrCatalogItem struct {
	Manifest     plugins.Manifest     `json:"Manifest"`
	ManifestPath string               `json:"ManifestPath"`
	ExecPath     string               `json:"ExecPath"`
	Specs        []sdk.AttrSpec       `json:"specs,omitempty"`
	Capabilities sdk.AttrCapabilities `json:"capabilities,omitempty"`
}

func New(cfg *config.Config) *Server {
	api := &Server{
		server:   sdk.NewServer("rpl", "core"),
		compiler: compilersvc.New(),
		language: languagesvc.New(cfg),
		types:    typecatalogsvc.New(),
	}

	api.registerHandlers()
	return api
}

func (api *Server) Run() error {
	if api == nil || api.server == nil {
		return nil
	}

	return api.server.Run()
}

func (api *Server) registerHandlers() {
	api.server.Handle("run", api.run)
	api.server.Handle("check", api.check)
	api.server.Handle("format", api.format)
	api.server.Handle("auto.set.import", api.autoSetImport)
	api.server.Handle("lang", api.setLanguage)
	api.server.Handle("lang.current", api.currentLanguage)

	api.server.Handle("attrs.get", api.attrInfo)
	api.server.Handle("attrs.search", api.searchAttrs)
	api.server.Handle(sdk.TypesCatalogAction, api.typeCatalog)
}

func (api *Server) format(msg sdk.Message) (any, error) {
	req, err := decodeFormatRequest(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	formatted, err := api.compiler.Format(req.Code, req.Path)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"code": formatted,
	}, nil
}

func (api *Server) autoSetImport(msg sdk.Message) (any, error) {
	req, err := decodeAutoImportRequest(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	updated, err := api.compiler.AutoSetImports(req.Code, req.Path)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"code": updated,
	}, nil
}

func (api *Server) run(msg sdk.Message) (any, error) {
	req, err := decodeRunRequest(msg.Value)
	if err != nil {
		return nil, err
	}

	file, err := api.compiler.Run(req.Code)
	if err != nil {
		return nil, err
	}

	return map[string]any{"ast": file}, nil
}

func (api *Server) check(msg sdk.Message) (any, error) {
	req, err := decodeCheckRequest(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	file, err := api.compiler.Check(req.Code, req.Path)
	if err != nil {
		return map[string]any{
			"ok":          false,
			"diagnostics": diagnosticsFromError(err),
		}, nil
	}

	return map[string]any{
		"ok":          true,
		"ast":         file,
		"diagnostics": []sdk.Diagnostic{},
	}, nil
}

func (api *Server) setLanguage(msg sdk.Message) (any, error) {
	req, err := decodeLanguageRequest(msg.Value)
	if err != nil {
		return nil, err
	}

	state, err := api.language.SetAt(req.Path, req.Lang)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"message": state.Message,
		"lang":    state.Lang,
	}, nil
}

func (api *Server) currentLanguage(msg sdk.Message) (any, error) {
	req, err := decodeLanguageRequest(msg.Value)
	if err != nil {
		return nil, err
	}
	return map[string]any{"lang": api.language.CurrentAt(req.Path)}, nil
}

func (api *Server) attrInfo(msg sdk.Message) (any, error) {
	req, err := decodeAttrLookup(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	attr, err := plugins.FindConfiguredAt(req.Path, req.Name, req.Author)
	if err != nil {
		return nil, err
	}

	description, err := plugins.DescribeAttrsAt(req.Path, attr.Manifest.Name, attr.Manifest.Author)
	if err != nil {
		description = sdk.DescribeAttrsResponse{}
	}
	capabilities, err := plugins.DescribeCapabilitiesAt(req.Path, attr.Manifest.Name, attr.Manifest.Author)
	if err != nil {
		capabilities = sdk.DescribeCapabilitiesResponse{}
	}

	return map[string]any{
		"attr": attrCatalogItem{
			Manifest:     attr.Manifest,
			ManifestPath: attr.ManifestPath,
			ExecPath:     attr.ExecPath,
			Specs:        description.Specs,
			Capabilities: capabilities.Capabilities,
		},
	}, nil
}

func (api *Server) searchAttrs(msg sdk.Message) (any, error) {
	req, err := decodeAttrSearch(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	items, err := plugins.ListConfiguredAt(req.Path)
	if err != nil {
		return nil, err
	}

	filtered := make([]attrCatalogItem, 0, len(items))
	query := strings.ToLower(strings.TrimSpace(req.Value))
	for _, item := range items {
		if query == "" || matchesAttrQuery(item, query) {
			description, err := plugins.DescribeAttrsAt(req.Path, item.Manifest.Name, item.Manifest.Author)
			if err != nil {
				description = sdk.DescribeAttrsResponse{}
			}
			capabilities, err := plugins.DescribeCapabilitiesAt(req.Path, item.Manifest.Name, item.Manifest.Author)
			if err != nil {
				capabilities = sdk.DescribeCapabilitiesResponse{}
			}
			filtered = append(filtered, attrCatalogItem{
				Manifest:     item.Manifest,
				ManifestPath: item.ManifestPath,
				ExecPath:     item.ExecPath,
				Specs:        description.Specs,
				Capabilities: capabilities.Capabilities,
			})
		}
	}

	sort.Slice(filtered, func(i int, j int) bool {
		if filtered[i].Manifest.Author == filtered[j].Manifest.Author {
			return filtered[i].Manifest.Name < filtered[j].Manifest.Name
		}

		return filtered[i].Manifest.Author < filtered[j].Manifest.Author
	})

	return map[string]any{
		"items": filtered,
		"count": len(filtered),
	}, nil
}

func (api *Server) typeCatalog(msg sdk.Message) (any, error) {
	req, err := decodeTypeCatalogRequest(msg.Value)
	if err != nil {
		return nil, err
	}
	if err := api.applyProjectConfig(req.Path); err != nil {
		return nil, err
	}

	catalog := sdk.TargetCatalog{}
	if api != nil && api.types != nil {
		catalog = api.types.Catalog(req.Lang)
	}

	return sdk.DescribeTargetCatalogResponse{
		Catalog: catalog.Normalized(),
	}, nil
}

func matchesAttrQuery(item plugins.Binary, query string) bool {
	return containsFold(item.Manifest.Name, query) ||
		containsFold(item.Manifest.Author, query) ||
		containsFold(item.Manifest.DisplayName, query) ||
		containsFold(item.Manifest.Description, query) ||
		containsFold(item.Manifest.Version, query) ||
		containsFold(item.ManifestPath, query) ||
		containsFold(item.ExecPath, query)
}

func containsFold(value string, query string) bool {
	return strings.Contains(strings.ToLower(value), query)
}

func (api *Server) applyProjectConfig(basePath string) error {
	if api == nil || api.language == nil || strings.TrimSpace(basePath) == "" {
		return nil
	}

	return api.language.ApplyAt(basePath)
}

func decodeRunRequest(value any) (runRequest, error) {
	if code, ok := value.(string); ok {
		return runRequest{Code: code}, nil
	}

	var req runRequest
	if err := sdk.Decode(value, &req); err != nil {
		return runRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeCheckRequest(value any) (checkRequest, error) {
	if code, ok := value.(string); ok {
		return checkRequest{Code: code}, nil
	}

	var req checkRequest
	if err := sdk.Decode(value, &req); err != nil {
		return checkRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeFormatRequest(value any) (formatRequest, error) {
	var req formatRequest
	if err := sdk.Decode(value, &req); err != nil {
		return formatRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeAutoImportRequest(value any) (autoImportRequest, error) {
	var req autoImportRequest
	if err := sdk.Decode(value, &req); err != nil {
		return autoImportRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeLanguageRequest(value any) (langRequest, error) {
	if lang, ok := value.(string); ok {
		return langRequest{Lang: lang}, nil
	}

	var req langRequest
	if err := sdk.Decode(value, &req); err != nil {
		return langRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeAttrLookup(value any) (attrLookupRequest, error) {
	var req attrLookupRequest
	if err := sdk.Decode(value, &req); err != nil {
		return attrLookupRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeAttrSearch(value any) (attrSearchRequest, error) {
	if text, ok := value.(string); ok {
		return attrSearchRequest{Value: text}, nil
	}

	var req attrSearchRequest
	if err := sdk.Decode(value, &req); err != nil {
		return attrSearchRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func decodeTypeCatalogRequest(value any) (typeCatalogRequest, error) {
	if text, ok := value.(string); ok {
		return typeCatalogRequest{Lang: text}, nil
	}

	var req typeCatalogRequest
	if err := sdk.Decode(value, &req); err != nil {
		return typeCatalogRequest{}, fmt.Errorf(localize.Text("некорректный запрос: %s", "invalid request: %s"), err.Error())
	}

	return req, nil
}

func diagnosticsFromError(err error) []sdk.Diagnostic {
	items := flattenErrors(err)
	diagnostics := make([]sdk.Diagnostic, 0, len(items))
	for _, item := range items {
		diagnostics = append(diagnostics, sdk.Diagnostic{
			Message: item.Message,
			Hint:    item.Hint,
			Detail:  strings.Join(item.Details, "\n"),
			Path:    item.FilePath,
			Line:    item.Line,
			Column:  item.Column,
		})
	}

	if len(diagnostics) == 0 && err != nil {
		diagnostics = append(diagnostics, sdk.Diagnostic{Message: err.Error()})
	}

	return diagnostics
}

func flattenErrors(err error) []*rplerr.Error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface {
		Unwrap() []error
	}

	if multi, ok := err.(multiUnwrapper); ok {
		items := make([]*rplerr.Error, 0)
		for _, item := range multi.Unwrap() {
			items = append(items, flattenErrors(item)...)
		}
		return items
	}

	var typed *rplerr.Error
	if errors.As(err, &typed) && typed != nil {
		return []*rplerr.Error{typed.Clone()}
	}

	return []*rplerr.Error{rplerr.New(err.Error())}
}
