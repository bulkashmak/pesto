package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto"
)

// cardSchema is the request schema the tests build on: POST /users/{id}/cards.
const cardSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["number", "kind", "limit"],
  "additionalProperties": false,
  "properties": {
    "number":  { "type": "string", "minLength": 16, "maxLength": 16, "examples": ["4111111111111111"] },
    "kind":    { "enum": ["debit", "credit"] },
    "limit":   { "type": "integer", "minimum": 0, "maximum": 10000, "default": 500 },
    "holder":  { "$ref": "#/$defs/holder" },
    "nickname": { "type": "string" }
  },
  "$defs": {
    "holder": {
      "type": "object",
      "required": ["name"],
      "properties": { "name": { "type": "string", "examples": ["Bob"] } }
    }
  }
}`

const cardResponseSchema = `{
  "type": "object",
  "required": ["id"],
  "properties": { "id": { "type": "integer" } }
}`

const cardEnvelope = `method: post
path: /users/{id}/cards
path_params:
  id: users.default.id
request:
  body: request.schema.json
response:
  success:
    status: 201
    body: response.schema.json
  on_violation:
    status: 422
`

// writeContract writes a contract folder and returns its envelope path.
func writeContract(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "contract.yaml")
}

func cardContract(t *testing.T) string {
	return writeContract(t, map[string]string{
		"contract.yaml":        cardEnvelope,
		"request.schema.json":  cardSchema,
		"response.schema.json": cardResponseSchema,
	})
}

func TestHappyPathFromSchemaValues(t *testing.T) {
	c, err := load(cardContract(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"number": "4111111111111111",
		"kind":   "debit",
		"limit":  float64(500),
		"holder": map[string]any{"name": "Bob"},
	}
	if !reflect.DeepEqual(c.happy, want) {
		t.Errorf("happy = %#v\nwant %#v", c.happy, want)
	}
}

func TestViolationsDerivedFromSchema(t *testing.T) {
	c, err := load(cardContract(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, v := range c.violations {
		names = append(names, v.name)
	}
	want := []string{
		"type/body",
		"additional_properties/body",
		"type/holder",
		"required/holder.name",
		"type/holder.name",
		"required/kind",
		"enum/kind",
		"required/limit",
		"type/limit",
		"minimum/limit",
		"maximum/limit",
		"required/number",
		"type/number",
		"min_length/number",
		"max_length/number",
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("violations =\n  %s\nwant\n  %s", strings.Join(names, "\n  "), strings.Join(want, "\n  "))
	}
	for _, v := range c.violations {
		if c.request.validate(v.body) == nil {
			t.Errorf("%s: body %v is valid, want a violation", v.name, v.body)
		}
	}
}

// TestViolationThatSchemaAllowsIsDropped checks that a type the schema also
// accepts is not reported as a violation.
func TestViolationThatSchemaAllowsIsDropped(t *testing.T) {
	s := writeContract(t, map[string]string{"s.json": `{
	  "type": "object",
	  "properties": { "note": { "type": ["string", "number", "boolean", "object", "array"], "examples": ["x"] } }
	}`})
	sch, err := loadSchema(filepath.Join(filepath.Dir(s), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	happy, err := sch.example(sch.root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range sch.violations(happy) {
		if strings.HasSuffix(v.name, "/note") {
			t.Errorf("unexpected violation %s", v.name)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]struct {
		envelope, schema, want string
	}{
		"missing example": {
			envelope: cardEnvelope,
			schema:   `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`,
			want:     `name: no example value: add "examples"`,
		},
		"example breaks schema": {
			envelope: cardEnvelope,
			schema:   `{"type":"object","properties":{"age":{"type":"integer","minimum":18,"examples":[3]}}}`,
			want:     "does not match the schema",
		},
		"bad method": {
			envelope: strings.Replace(cardEnvelope, "method: post", "method: fetch", 1),
			schema:   cardSchema,
			want:     `method "FETCH" must be one of`,
		},
		"unmapped param": {
			envelope: strings.Replace(cardEnvelope, "  id: users.default.id\n", "  other: x\n", 1),
			schema:   cardSchema,
			want:     "path parameter {id} has no entry in path_params",
		},
		"unknown key": {
			envelope: cardEnvelope + "retries: 3\n",
			schema:   cardSchema,
			want:     "field retries not found",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeContract(t, map[string]string{
				"contract.yaml":        tc.envelope,
				"request.schema.json":  tc.schema,
				"response.schema.json": cardResponseSchema,
			})
			_, err := load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("load error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// cardServer is a fake API for POST /users/{id}/cards. A strict server rejects
// every schema violation with 422; a sloppy one forgets to require "limit".
func cardServer(t *testing.T, strict bool) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users/42/cards", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&body); err != nil || !validCard(body, strict) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 7}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func validCard(b map[string]any, strict bool) bool {
	for k := range b {
		switch k {
		case "number", "kind", "limit", "holder", "nickname":
		default:
			return false
		}
	}
	num, ok := b["number"].(string)
	if !ok || len(num) != 16 {
		return false
	}
	if kind, _ := b["kind"].(string); kind != "debit" && kind != "credit" {
		return false
	}
	if limit, present := b["limit"]; present || strict {
		f, ok := limit.(float64)
		if !ok || f != float64(int64(f)) || f < 0 || f > 10000 {
			return false
		}
	}
	if h, present := b["holder"]; present {
		hm, ok := h.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := hm["name"].(string); !ok {
			return false
		}
	}
	return true
}

func TestRunAgainstStrictServer(t *testing.T) {
	srv := cardServer(t, true)
	pt := pesto.New(t, pesto.WithBaseURL(srv.URL), pesto.WithData(cardData))
	Run(pt, cardContract(t))
}

// TestRunCatchesSloppyServer runs a contract against a server that accepts a
// missing required field, in a child process, and checks that exactly that
// violation fails.
func TestRunCatchesSloppyServer(t *testing.T) {
	if os.Getenv("PESTO_CONTRACT_SLOPPY") == "1" {
		srv := cardServer(t, false)
		pt := pesto.New(t, pesto.WithBaseURL(srv.URL), pesto.WithData(cardData))
		Run(pt, cardContract(t))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunCatchesSloppyServer$", "-test.v")
	cmd.Env = append(os.Environ(), "PESTO_CONTRACT_SLOPPY=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("contract passed against a sloppy server:\n%s", out)
	}
	got := string(out)
	if !strings.Contains(got, "--- FAIL: TestRunCatchesSloppyServer/violation/required/limit") {
		t.Errorf("missing failure for required/limit:\n%s", got)
	}
	if strings.Count(got, "--- FAIL: TestRunCatchesSloppyServer/") != 1 {
		t.Errorf("want exactly one failing case:\n%s", got)
	}
	if !strings.Contains(got, "status = 201, want 422") {
		t.Errorf("failure message lacks the status:\n%s", got)
	}
}

// cardData is the static data the card contract's path parameter reads.
var cardData = map[string]any{
	"users": map[string]any{"default": map[string]any{"id": 42}},
}

// TestRunMissingData checks, in a child process, that a path parameter whose
// data reference is missing fails the contract before any request, naming the
// parameter and the reason.
func TestRunMissingData(t *testing.T) {
	if os.Getenv("PESTO_CONTRACT_NODATA") == "1" {
		srv := cardServer(t, true)
		Run(pesto.New(t, pesto.WithBaseURL(srv.URL)), cardContract(t))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunMissingData$", "-test.v")
	cmd.Env = append(os.Environ(), "PESTO_CONTRACT_NODATA=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("contract passed with no data:\n%s", out)
	}
	got := string(out)
	for _, want := range []string{
		"path parameter {id} reads data users.default.id",
		`pesto: data users.default.id: no data file "users"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "happy_path") {
		t.Errorf("cases ran despite the missing data:\n%s", got)
	}
}
