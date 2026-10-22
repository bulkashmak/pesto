package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto"
)

// accountCardSchema has fields only static data can fill: account_id must be
// an account that exists, and holder.user_id has no example at all.
const accountCardSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["account_id", "number"],
  "additionalProperties": false,
  "properties": {
    "account_id": { "type": "integer", "minimum": 1 },
    "number":     { "type": "string", "minLength": 16, "maxLength": 16, "examples": ["4111111111111111"] },
    "holder": {
      "type": "object",
      "required": ["user_id", "name"],
      "properties": {
        "user_id": { "type": "string" },
        "name":    { "type": "string", "examples": ["Bob"] }
      }
    },
    "active": { "type": "boolean" },
    "rate":   { "type": "number" },
    "tags":   { "type": "array", "items": { "type": "string" } }
  }
}`

const accountCardEnvelope = `method: POST
path: /cards
request:
  body: request.schema.json
  data:
    account_id: accounts.default.id
    holder.user_id: users.default.id
    active: accounts.default.active
    rate: accounts.default.rate
response:
  success:
    status: 201
  on_violation:
    status: 422
`

// accountCardData is the static data accountCardEnvelope reads.
var accountCardData = map[string]any{
	"accounts": map[string]any{"default": map[string]any{"id": 42, "active": true, "rate": 1.5}},
	"users":    map[string]any{"default": map[string]any{"id": "u-7"}},
}

// accountCardValues is accountCardData as the text values fill receives.
var accountCardValues = map[string]string{
	"account_id":     "42",
	"holder.user_id": "u-7",
	"active":         "true",
	"rate":           "1.5",
}

func accountCardContract(t *testing.T, envelope string) string {
	return writeContract(t, map[string]string{
		"contract.yaml":       envelope,
		"request.schema.json": accountCardSchema,
	})
}

// asJSON renders v for comparisons that should not care about Go number types.
func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestDataFillsHappyPath checks that request.data values land in the body with
// their schema types, and that the optional holder object is created for its
// data field.
func TestDataFillsHappyPath(t *testing.T) {
	c, err := load(accountCardContract(t, accountCardEnvelope))
	if err != nil {
		t.Fatal(err)
	}
	if c.happy != nil || c.violations != nil {
		t.Fatal("cases derived before the data was filled")
	}
	if err := c.fill(accountCardValues); err != nil {
		t.Fatal(err)
	}
	want := `{"account_id":42,"active":true,"holder":{"name":"Bob","user_id":"u-7"},"number":"4111111111111111","rate":1.5}`
	if got := asJSON(t, c.happy); got != want {
		t.Errorf("happy = %s\nwant    %s", got, want)
	}
}

// TestDataViolationsStartFromFilledBody checks that data fields get their own
// violations and that every other violation keeps the data value.
func TestDataViolationsStartFromFilledBody(t *testing.T) {
	c, err := load(accountCardContract(t, accountCardEnvelope))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.fill(accountCardValues); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, v := range c.violations {
		names[v.name] = true
		if strings.HasSuffix(v.name, "/account_id") || v.name == "type/body" {
			continue
		}
		if got := lookup(v.body, path{"account_id"}); got != int64(42) {
			t.Errorf("%s: account_id = %#v, want the data value 42", v.name, got)
		}
	}
	for _, want := range []string{"required/account_id", "type/account_id", "minimum/account_id", "type/holder.user_id", "required/number"} {
		if !names[want] {
			t.Errorf("missing violation %s (have %v)", want, names)
		}
	}
}

// TestDataLoadErrors covers request.data entries rejected when the contract
// is loaded, before any data is read.
func TestDataLoadErrors(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(accountCardEnvelope, old, new, 1) }
	cases := map[string]struct{ envelope, want string }{
		"unknown field": {
			replace("    account_id: accounts", "    acount_id: accounts"),
			"request.schema.json declares no property acount_id",
		},
		"unknown nested field": {
			replace("    holder.user_id:", "    holder.userid:"),
			"request.schema.json declares no property holder.userid",
		},
		"object field": {
			replace("    holder.user_id:", "    holder:"),
			"request.data.holder: holder is not a single value in",
		},
		"array field": {
			replace("    rate:", "    tags:"),
			"request.data.tags: tags is not a single value",
		},
		"through a value": {
			replace("    rate:", "    number.x:"),
			"request.data.number.x: number is not an object",
		},
		"empty segment": {
			replace("    holder.user_id:", "    holder..user_id:"),
			`request.data: invalid field "holder..user_id"`,
		},
		"no body": {
			replace("  body: request.schema.json\n", ""),
			"request.data needs request.body",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(accountCardContract(t, tc.envelope))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("load error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestDataFillErrors covers data values that do not fit their field.
func TestDataFillErrors(t *testing.T) {
	cases := map[string]struct {
		field, value, want string
	}{
		"not an integer": {"account_id", "abc", `request.data.account_id: data accounts.default.id: "abc" is not a valid integer`},
		"not a boolean":  {"active", "yes please", `request.data.active: data accounts.default.active: "yes please" is not a valid boolean`},
		"breaks schema":  {"account_id", "0", "built from the examples and request.data in"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := load(accountCardContract(t, accountCardEnvelope))
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]string{}
			for k, v := range accountCardValues {
				values[k] = v
			}
			values[tc.field] = tc.value
			if err := c.fill(values); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("fill error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// accountCardServer accepts exactly the bodies accountCardSchema allows, with
// account 42 and user u-7 the only ones that exist.
func accountCardServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil || !validAccountCard(b) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func validAccountCard(b map[string]any) bool {
	for k := range b {
		switch k {
		case "account_id", "number", "holder", "active", "rate", "tags":
		default:
			return false
		}
	}
	if id, ok := b["account_id"].(float64); !ok || id != 42 {
		return false
	}
	if num, ok := b["number"].(string); !ok || len(num) != 16 {
		return false
	}
	if h, present := b["holder"]; present {
		hm, ok := h.(map[string]any)
		if !ok || hm["user_id"] != "u-7" {
			return false
		}
		if _, ok := hm["name"].(string); !ok {
			return false
		}
	}
	if a, present := b["active"]; present {
		if _, ok := a.(bool); !ok {
			return false
		}
	}
	if r, present := b["rate"]; present {
		if _, ok := r.(float64); !ok {
			return false
		}
	}
	return true
}

// TestRunWithData runs a request.data contract end to end against a server
// that only knows the account and user the data names.
func TestRunWithData(t *testing.T) {
	srv := accountCardServer(t)
	pt := pesto.New(t, pesto.WithBaseURL(srv.URL), pesto.WithData(accountCardData))
	Run(pt, accountCardContract(t, accountCardEnvelope))
}

// TestRunMissingBodyData checks, in a child process, that a missing
// request.data reference fails the contract before any request, naming the
// field and the reason.
func TestRunMissingBodyData(t *testing.T) {
	if os.Getenv("PESTO_CONTRACT_NOBODYDATA") == "1" {
		srv := accountCardServer(t)
		data := map[string]any{"accounts": accountCardData["accounts"]} // no users file
		Run(pesto.New(t, pesto.WithBaseURL(srv.URL), pesto.WithData(data)), accountCardContract(t, accountCardEnvelope))
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunMissingBodyData$", "-test.v")
	cmd.Env = append(os.Environ(), "PESTO_CONTRACT_NOBODYDATA=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("contract passed with missing data:\n%s", out)
	}
	got := string(out)
	for _, want := range []string{
		"request field holder.user_id reads data users.default.id",
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
