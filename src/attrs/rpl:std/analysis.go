package main

import "rpl/pkg/sdk"

var stdAttrSpecs = []sdk.AttrSpec{
	{
		Namespace: "group",
		Help:      sdk.Text("Собирает отдельную сгенерированную модель из полей текущей модели с одинаковой группой.", "Builds a separate generated model from fields of the current model that share the same group."),
		Args: []sdk.AttrArgSpec{
			{Name: "value", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Positional: true, Help: sdk.Text("Имя группы, например data или req.", "Group name, for example data or req.")},
		},
		Snippets: []sdk.AttrSnippetSpec{
			{Label: "@group", Insert: "@group", Help: sdk.Text("Группирует поле в отдельную производную модель.", "Groups a field into a separate derived model.")},
		},
	},
	{
		Namespace: "comment",
		Help:      sdk.Text("Добавляет человекочитаемое описание к модели.", "Adds a human-readable description to a model."),
		Args: []sdk.AttrArgSpec{
			{Name: "value", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Positional: true, Help: sdk.Text("Текст комментария.", "Comment text.")},
		},
		Snippets: []sdk.AttrSnippetSpec{
			{Label: "@comment", Insert: "@comment", Help: sdk.Text("Добавляет комментарий к модели.", "Adds a comment to a model.")},
		},
	},
	{
		Namespace: "ignore",
		Help:      sdk.Text("Исключает поле из конкретных attrs, например grpc или sql.", "Excludes a field from specific attrs such as grpc or sql."),
		Args: []sdk.AttrArgSpec{
			{Name: "value", Types: []sdk.AttrValueType{sdk.AttrValueTypeStringLike}, Positional: true, Help: sdk.Text("Имя attr или список attr-ов.", "Attr name or attr list.")},
		},
		Snippets: []sdk.AttrSnippetSpec{
			{Label: "@ignore", Insert: "@ignore", Help: sdk.Text("Исключает поле из выбранных attrs.", "Excludes a field from selected attrs.")},
		},
	},
}

func analyzeStd(req sdk.GenerateRequest) (sdk.AnalyzeResponse, error) {
	_ = req
	return sdk.AnalyzeResponse{}, nil
}
