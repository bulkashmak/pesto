package contract

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// envelope is the contract.yaml file: the HTTP parts of a contract that JSON
// Schema has no concept of.
type envelope struct {
	Method     string            `yaml:"method"`
	Path       string            `yaml:"path"`
	PathParams map[string]string `yaml:"path_params"`
	Request    struct {
		Body string            `yaml:"body"`
		Data map[string]string `yaml:"data"` // body field path -> data reference
	} `yaml:"request"`
	Response struct {
		Success struct {
			Status int    `yaml:"status"`
			Body   string `yaml:"body"`
		} `yaml:"success"`
		OnViolation struct {
			Status int `yaml:"status"`
		} `yaml:"on_violation"`
	} `yaml:"response"`
}

// methods are the HTTP methods a contract may use.
var methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// paramRE matches a {param} placeholder in a contract path.
var paramRE = regexp.MustCompile(`\{([^{}/]+)\}`)

// loadEnvelope reads and validates the envelope at path. Unknown keys are an
// error, so a typo does not silently drop an expectation.
func loadEnvelope(path string) (*envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var env envelope
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := env.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &env, nil
}

func (e *envelope) validate() error {
	e.Method = strings.ToUpper(strings.TrimSpace(e.Method))
	switch {
	case !slices.Contains(methods, e.Method):
		return fmt.Errorf("method %q must be one of %s", e.Method, strings.Join(methods, ", "))
	case !strings.HasPrefix(e.Path, "/"):
		return fmt.Errorf("path %q must start with /", e.Path)
	case e.Response.Success.Status == 0:
		return errors.New("response.success.status is required")
	case e.Request.Body != "" && e.Response.OnViolation.Status == 0:
		return errors.New("response.on_violation.status is required when request.body is set")
	case len(e.Request.Data) > 0 && e.Request.Body == "":
		return errors.New("request.data needs request.body: it fills fields of the request body")
	}
	for field, ref := range e.Request.Data {
		if field == "" || slices.Contains(strings.Split(field, "."), "") {
			return fmt.Errorf("request.data: invalid field %q: want a dotted path, e.g. holder.user_id", field)
		}
		if ref == "" {
			return fmt.Errorf("request.data.%s: no data reference", field)
		}
	}

	params := map[string]bool{}
	for _, m := range paramRE.FindAllStringSubmatch(e.Path, -1) {
		params[m[1]] = true
		if e.PathParams[m[1]] == "" {
			return fmt.Errorf("path parameter {%s} has no entry in path_params", m[1])
		}
	}
	for name := range e.PathParams {
		if !params[name] {
			return fmt.Errorf("path_params.%s does not appear in path %q", name, e.Path)
		}
	}
	return nil
}

// resolve returns path relative to the envelope's directory.
func resolve(envelopePath, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(envelopePath), path)
}
