package transform

import (
	"strings"
	"testing"
)

const sampleSrc = `package users

import (
	"github.com/bulkashmak/pesto"
)

// TestCreateUser creates a user.
func TestCreateUser(t *pesto.T) {
	t.Client.POST("/users").Send().Expect().Status(201)
}

func helper() {}

func TestGetUser(t *pesto.T) {
	t.Client.GET("/users/1").Send().Expect().Status(200)
}
`

func TestRewriteFileFindsAndRenames(t *testing.T) {
	res, err := RewriteFile("/abs/users_test.go", []byte(sampleSrc))
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "users" {
		t.Errorf("package = %q, want users", res.Package)
	}
	if len(res.Tests) != 2 {
		t.Fatalf("found %d tests, want 2: %+v", len(res.Tests), res.Tests)
	}

	got := string(res.Rewritten)
	for _, want := range []string{
		"//line /abs/users_test.go:1",
		"func pestoBody_TestCreateUser(t *pesto.T)",
		"func pestoBody_TestGetUser(t *pesto.T)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten source missing %q\n---\n%s", want, got)
		}
	}
	// The original exported names must be gone from declarations (they move to
	// the generated wrappers file instead).
	if strings.Contains(got, "func TestCreateUser(") {
		t.Errorf("original test declaration was not renamed:\n%s", got)
	}
	// Non-test funcs and bodies are untouched.
	if !strings.Contains(got, "func helper() {}") {
		t.Errorf("helper() should be untouched:\n%s", got)
	}
}

func TestRewriteFileNoPestoTests(t *testing.T) {
	src := `package x

import "testing"

func TestPlain(t *testing.T) {}
`
	res, err := RewriteFile("/abs/x_test.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tests) != 0 || res.Rewritten != nil {
		t.Errorf("expected no pesto tests, got %+v", res)
	}
}

func TestWrappersFile(t *testing.T) {
	out, err := WrappersFile("users", []TestFunc{{Name: "TestGetUser"}, {Name: "TestCreateUser"}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"package users",
		`pesto "github.com/bulkashmak/pesto"`,
		"pesto.Run(t, pestoBody_TestCreateUser)",
		"pesto.Run(t, pestoBody_TestGetUser)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wrappers missing %q\n---\n%s", want, got)
		}
	}
	// Sorted deterministically: CreateUser before GetUser.
	if strings.Index(got, "TestCreateUser") > strings.Index(got, "TestGetUser") {
		t.Errorf("wrappers not sorted deterministically:\n%s", got)
	}
}
