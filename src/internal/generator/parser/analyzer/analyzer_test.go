package analyzer_test

import (
	"strings"
	"testing"

	"rpl/internal/generator/parser"
	"rpl/internal/generator/parser/lexer"
	rplerr "rpl/pkg/error"
)

func TestFinalizeFileRejectsDuplicateFieldsAndMissingFieldExtensionTarget(t *testing.T) {
	err := finalizeCode(t, `model User {
    Name string
    Name int
}

field User.Missing {
    @comment("boom")
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `duplicate field "Name"`)
	assertContains(t, rendered, `field "Missing" of model "User"`)
}

func TestFinalizeFileRejectsUndeclaredAttrNamespaces(t *testing.T) {
	err := finalizeCode(t, `attrs (
    "rpl:sql"
)

model User {
    Name int @validate(email: true)
    Login string @foo(enabled: true)
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `attr namespace "validate" is not declared`)
	assertContains(t, rendered, `attr namespace "foo" is not declared`)
}

func TestFinalizeFileRejectsMissingModelReferences(t *testing.T) {
	err := finalizeCode(t, `model Request {
    Profile Profile
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `references a missing model "Profile"`)
}

func TestFinalizeFileRejectsAutoGroupConflicts(t *testing.T) {
	err := finalizeCode(t, `model UserReq {}

model User {
    Name string @group("req")
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `auto-group "UserReq" conflicts`)
}

func TestFinalizeFileRejectsFilenameCollisionsAndUnusedImports(t *testing.T) {
	err := finalizeCode(t, `import (
    "time"
)

attrs (
    "rpl:sql"
)

@sql(db: "postgres", table: "users")
model User {
    ID int
}

@sql(db: "postgres", table: "users2")
model user {
    ID int
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `unused import "time"`)
	assertContains(t, rendered, `collision in generated filename "user/model.gen.go"`)
}

func TestFinalizeFileRejectsUnresolvedMethodTypes(t *testing.T) {
	err := finalizeCode(t, `attrs (
    "rpl:grpc"
)

model User {
    Conn string (
        func Open (conn socket.Unknown) return (error) @grpc.Inside()
    )
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `argument "conn" of method "Open"`)
	assertContains(t, rendered, `unresolved external Go type "socket.Unknown"`)
}

func TestFinalizeFileAcceptsModelFieldTypeReferencesInMethodSignatures(t *testing.T) {
	err := finalizeCode(t, `attrs (
    "rpl:grpc"
)

model User {
    Name string (
        func Ping return (User.Name)
    )
}
`)
	if err != nil {
		t.Fatalf("expected method signature to accept Model.Field reference, got: %v", err)
	}
}

func TestFinalizeFileAcceptsTypeAliases(t *testing.T) {
	err := finalizeCode(t, `import (
    "time"
)

type Email string
type Timestamp time.Time

model User {
    Email Email
    CreatedAt Timestamp
}
`)
	if err != nil {
		t.Fatalf("expected type aliases to be accepted, got: %v", err)
	}
}

func TestFinalizeFileRejectsTypeAliasesToModels(t *testing.T) {
	err := finalizeCode(t, `model User {
    Name string
}

type UserRef User
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `type "UserRef" cannot alias model "User"`)
}

func TestFinalizeFileAcceptsScalarGroupedFields(t *testing.T) {
	err := finalizeCode(t, `attrs (
    "rpl:std"
)

model User {
    Name string @group("data")
    Age int @group("data")
}

model Request {
    User UserData
}
`)
	if err != nil {
		t.Fatalf("expected grouped scalar fields to be accepted, got: %v", err)
	}
}

func TestFinalizeFileAcceptsMatchingDuplicateTargets(t *testing.T) {
	err := finalizeCode(t, `target(lang: golang)

target(lang: golang)

model User {
    Name string
}
`)
	if err != nil {
		t.Fatalf("expected matching duplicate targets to be accepted, got: %v", err)
	}
}

func TestFinalizeFileRejectsConflictingDuplicateTargets(t *testing.T) {
	err := finalizeCode(t, `target(lang: golang)

target(lang: rust)

model User {
    Name string
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `target directives inside one package must match`)
}

func TestFinalizeFileTreatsUppercaseModelFieldRefsAsModelFieldErrors(t *testing.T) {
	err := finalizeCode(t, `model User2 {
    Id User.Id
}
`)
	if err == nil {
		t.Fatal("expected error")
	}

	rendered := rplerr.Format(nil, err)
	assertContains(t, rendered, `references a missing model field "User.Id"`)
}

func finalizeCode(t *testing.T, code string) error {
	t.Helper()

	lex := lexer.NewLexerWithPath(code, "/tmp/schema.rpl")
	if err := lex.Run(); err != nil {
		return err
	}

	file, err := parser.New(lex).Parse()
	if err != nil {
		return err
	}

	return parser.FinalizeFile(file)
}

func assertContains(t *testing.T, body string, want string) {
	t.Helper()

	if !strings.Contains(body, want) {
		t.Fatalf("expected %q in error output\n%s", want, body)
	}
}
