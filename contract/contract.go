package contract

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/bulkashmak/pesto"
)

// Run loads the contract at envelopePath (usually "contract.yaml", resolved
// against the test's package directory) and runs its cases as subtests:
// happy_path, then one violation/<kind>/<field> per derived violation.
//
// Path parameters and the request body fields listed under request.data are
// filled from the environment's static test data via [pesto.T.Data]. A
// contract that cannot be loaded or built - an invalid envelope or schema, a
// missing data reference, a required field with no example - fails the test
// before any request is sent.
func Run(t *pesto.T, envelopePath string) {
	t.Helper()

	c, err := load(envelopePath)
	if err != nil {
		t.Fatalf("pesto: contract %s: %v", envelopePath, err)
	}
	target := c.target(t, envelopePath)
	if len(c.env.Request.Data) > 0 {
		if err := c.fill(c.bodyData(t, envelopePath)); err != nil {
			t.Fatalf("pesto: contract %s: %v", envelopePath, err)
		}
	}

	t.Run("happy_path", func(t *pesto.T) {
		resp := c.send(t, target, c.happy)
		if resp.Status() != c.env.Response.Success.Status {
			t.Errorf("status = %d, want %d\n%s", resp.Status(), c.env.Response.Success.Status, exchange(c.happy, c.request != nil, resp))
			return
		}
		if c.response != nil {
			if err := c.response.validateJSON(resp.Body()); err != nil {
				t.Errorf("response body does not match %s: %v\nresponse body: %s", c.response.file, err, resp.Body())
			}
		}
	})

	for _, v := range c.violations {
		t.Run("violation/"+v.name, func(t *pesto.T) {
			resp := c.send(t, target, v.body)
			if want := c.env.Response.OnViolation.Status; resp.Status() != want {
				t.Errorf("status = %d, want %d for a request that breaks the schema (%s)\n%s",
					resp.Status(), want, v.name, exchange(v.body, true, resp))
			}
		})
	}
}

// contractCase is a loaded contract: the envelope, its schemas, and the cases
// derived from them. With request.data, the cases exist only once [fill] has
// put the data into the body.
type contractCase struct {
	env        *envelope
	request    *schema                   // nil when the request has no body
	response   *schema                   // nil when the success body is not checked
	fields     map[string]map[string]any // request.data field -> its schema node
	template   any                       // happy body with request.data fields as slots
	happy      any
	violations []violation
}

// load reads the envelope and its schemas and derives every case.
func load(envelopePath string) (*contractCase, error) {
	env, err := loadEnvelope(envelopePath)
	if err != nil {
		return nil, err
	}
	c := &contractCase{env: env}

	if env.Request.Body != "" {
		if c.request, err = loadSchema(resolve(envelopePath, env.Request.Body)); err != nil {
			return nil, err
		}
		slots := map[string]bool{}
		c.fields = map[string]map[string]any{}
		for field := range env.Request.Data {
			node, err := c.request.field(strings.Split(field, "."))
			if err != nil {
				return nil, fmt.Errorf("request.data.%s: %w", field, err)
			}
			slots[field] = true
			c.fields[field] = node
		}
		if c.template, err = c.request.example(c.request.root, nil, slots); err != nil {
			return nil, fmt.Errorf("build the happy-path request from %s: %w", c.request.file, err)
		}
		if len(slots) == 0 {
			if err := c.fill(nil); err != nil {
				return nil, err
			}
		}
	}
	if env.Response.Success.Body != "" {
		if c.response, err = loadSchema(resolve(envelopePath, env.Response.Success.Body)); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// fill sets each request.data field of the body template from values (field ->
// data value), converted to the field's schema type, then validates the body
// and derives the violations from it - so every violation changes one field
// of a body whose data is valid.
func (c *contractCase) fill(values map[string]string) error {
	body := clone(c.template)
	for _, field := range sortedKeys(c.fields) {
		v, err := convert(values[field], c.fields[field])
		if err != nil {
			return fmt.Errorf("request.data.%s: data %s: %w", field, c.env.Request.Data[field], err)
		}
		if err := setPath(body, strings.Split(field, "."), v); err != nil {
			return fmt.Errorf("request.data.%s: %w", field, err)
		}
	}
	if err := c.request.validate(body); err != nil {
		from := "the examples"
		if len(c.fields) > 0 {
			from = "the examples and request.data"
		}
		return fmt.Errorf("the happy-path request built from %s in %s does not match the schema: %w", from, c.request.file, err)
	}
	c.happy = body
	c.violations = c.request.violations(body)
	return nil
}

// bodyData reads every request.data reference from the test's static data. A
// missing reference fails the test, naming the field and what is missing.
func (c *contractCase) bodyData(t *pesto.T, envelopePath string) map[string]string {
	t.Helper()
	values := map[string]string{}
	for _, field := range sortedKeys(c.env.Request.Data) {
		ref := c.env.Request.Data[field]
		if _, ok := t.LookupData(ref); !ok {
			t.Errorf("pesto: contract %s: request field %s reads data %s", envelopePath, field, ref)
		}
		values[field] = t.Data(ref) // fails with the reason when missing
	}
	return values
}

// target fills the path parameters from the test's static data. A missing
// reference fails the test, naming the parameter and what is missing.
func (c *contractCase) target(t *pesto.T, envelopePath string) string {
	t.Helper()
	return paramRE.ReplaceAllStringFunc(c.env.Path, func(m string) string {
		param := m[1 : len(m)-1]
		ref := c.env.PathParams[param]
		if _, ok := t.LookupData(ref); !ok {
			t.Errorf("pesto: contract %s: path parameter {%s} reads data %s", envelopePath, param, ref)
		}
		return url.PathEscape(t.Data(ref)) // fails with the reason when missing
	})
}

// send issues the contract's request with body (no body when the contract has
// no request schema).
func (c *contractCase) send(t *pesto.T, target string, body any) *pesto.Response {
	t.Helper()
	req := t.Client.Do(c.env.Method, target)
	if c.request != nil {
		req = req.JSON(body)
	}
	return req.Send()
}

// exchange renders the request and response bodies for a failure message.
func exchange(body any, hasBody bool, resp *pesto.Response) string {
	var b strings.Builder
	if hasBody {
		sent, _ := json.Marshal(body)
		fmt.Fprintf(&b, "request body:  %s\n", sent)
	}
	fmt.Fprintf(&b, "response body: %s", resp.Body())
	return b.String()
}
