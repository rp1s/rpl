package target_test

import (
	"rpl/internal/generator/parser"
	"rpl/internal/generator/parser/ast"
	"rpl/internal/generator/parser/lexer"
	targetpkg "rpl/internal/generator/target"
	"rpl/pkg/sdk"
	"strings"
	"testing"
)

func TestFacadeCodeUsesZeroValueWhenModelDefaultsAreNotGenerated(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

import (
    "time"
)

model User {
    CreatedAt time.Time = now
}
`)
	model := mustFindModel(t, file, "User")

	baseCode := targetpkg.Golang{}.BaseModelCode(file, model)
	facadeCode := targetpkg.Golang{}.FacadeCode(file, model, "example.com/acme/app/models/user", "user")

	if strings.Contains(baseCode, "func DefaultUser() User") {
		t.Fatalf("base model unexpectedly generated defaults helper:\n%s", baseCode)
	}
	if !strings.Contains(facadeCode, "return WrapUser(user.User{})") {
		t.Fatalf("facade should fall back to zero value when subpackage default helper is absent:\n%s", facadeCode)
	}
	if strings.Contains(facadeCode, "user.DefaultUser()") {
		t.Fatalf("facade must not call missing subpackage default helper:\n%s", facadeCode)
	}
}

func TestFacadeUsesUniqueModelImportAliasForWebsocketLikeModel(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

import (
    websocket "github.com/gorilla/websocket"
)

attrs (
    "rpl:grpc"
)

@grpc
model Websocket {
    Websocket websocket.Conn {
        @grpc.Inside
    }
}
`)
	model := mustFindModel(t, file, "Websocket")

	imports := targetpkg.Golang{}.FacadeImports(file, model, "example.com/acme/app/models/websocket", "websocket")
	facadeCode := targetpkg.Golang{}.FacadeCode(file, model, "example.com/acme/app/models/websocket", "websocket")

	assertImportRef(t, imports, "modelpkg", "example.com/acme/app/models/websocket")
	if !strings.Contains(facadeCode, "return WrapWebsocket(modelpkg.Websocket{})") {
		t.Fatalf("facade should use the resolved model alias and zero-value default:\n%s", facadeCode)
	}
	if strings.Contains(facadeCode, "return WrapWebsocket(websocket.DefaultWebsocket())") {
		t.Fatalf("facade must not call a missing default helper through a conflicting alias:\n%s", facadeCode)
	}
}

func TestBaseModelResolvesRecursiveModelFieldRefsAndImports(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

import (
    uuid "github.com/google/uuid"
)

model Base {
    Id uuid.UUID
}

model User {
    Id Base.Id
}

model User2 {
    Id User.Id
}
`)
	model := mustFindModel(t, file, "User2")

	baseCode := targetpkg.Golang{}.BaseModelCode(file, model)
	imports := targetpkg.Golang{}.UsedImports(file, model)

	if !strings.Contains(baseCode, "Id uuid.UUID") {
		t.Fatalf("base model should resolve recursive model-field refs to the concrete imported type:\n%s", baseCode)
	}
	assertImportRef(t, imports, "uuid", "github.com/google/uuid")
}

func TestBaseModelSupportsGoExpressionDefaultsAndImports(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

import (
    "math/bits"
    "time"
)

model User {
    Size int = bits.UintSize
    Tags []string = []string{"a", "b"}
    CreatedAt time.Time? = time.Now()
}
`)
	model := mustFindModel(t, file, "User")

	baseCode := targetpkg.Golang{}.BaseModelCode(file, model)
	imports := targetpkg.Golang{}.UsedImports(file, model)

	for _, want := range []string{
		"func DefaultUser() User",
		"Size: bits.UintSize,",
		`Tags: []string{"a", "b"},`,
		"CreatedAt: func() *time.Time { value := time.Now(); return &value }(),",
	} {
		if !strings.Contains(baseCode, want) {
			t.Fatalf("expected generated default helper to contain %q, got:\n%s", want, baseCode)
		}
	}

	assertImportRef(t, imports, "bits", "math/bits")
	assertImportRef(t, imports, "time", "time")
}

func TestBaseModelReexportsLocalTypeAliases(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

type Email string

model User {
    Email Email
}
`)
	model := mustFindModel(t, file, "User")

	baseCode := targetpkg.Golang{}.BaseModelCode(file, model)

	for _, want := range []string{
		"type Email = typespkg.Email",
		"type User struct",
		"Email Email",
	} {
		if !strings.Contains(baseCode, want) {
			t.Fatalf("expected generated base model to contain %q, got:\n%s", want, baseCode)
		}
	}
}

func TestBaseModelGeneratesMethodContracts(t *testing.T) {
	file := parseRawFile(t, `target(lang: golang)

import (
    "time"
)

model Profile {
    Name string
}

model Position {
    X int64
    Z int64? (
        func Lift(delta int64) return (Position)
    )

    func Is2d return (bool)
    func Touch(at time.Time) return (Profile, error)
}
`)
	model := mustFindModel(t, file, "Position")

	baseCode := targetpkg.Golang{}.BaseModelCode(file, model)
	imports := targetpkg.Golang{}.UsedImports(file, model)

	for _, want := range []string{
		"type PositionMethods interface {",
		"Is2d(model *Position) bool",
		"Touch(model *Position, at time.Time) (profile.Profile, error)",
		"var (",
		"positionMethodsImpl PositionMethods",
		"func SetPositionMethods(methods PositionMethods)",
		"func (model *Position) Is2d() bool",
		"return methods.Is2d(model)",
		"type PositionZMethods interface {",
		"Lift(delta int64) Position",
	} {
		if !strings.Contains(baseCode, want) {
			t.Fatalf("expected generated base model to contain %q, got:\n%s", want, baseCode)
		}
	}

	assertImportRef(t, imports, "time", "time")
}

func parseRawFile(t *testing.T, body string) *ast.File {
	t.Helper()

	lex := lexer.NewLexerWithPath(body, "/tmp/schema.rpl")
	if err := lex.Run(); err != nil {
		t.Fatalf("lex file: %v", err)
	}

	file, err := parser.New(lex).Parse()
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	return file
}

func mustFindModel(t *testing.T, file *ast.File, name string) *ast.ModelAST {
	t.Helper()

	model, ok := file.FindModel(name)
	if !ok || model == nil {
		t.Fatalf("model %q not found", name)
	}

	return model
}

func assertImportRef(t *testing.T, imports []sdk.ImportRef, alias string, path string) {
	t.Helper()

	for _, item := range imports {
		if item.Alias == alias && item.Path == path {
			return
		}
	}

	t.Fatalf("import %q %q not found in %+v", alias, path, imports)
}
