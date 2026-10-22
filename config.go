package pesto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// defaultEnvironmentsDir is where environment files live when pesto.yaml does not
// override it.
const defaultEnvironmentsDir = "environments"

// Config is the project-level configuration, read from pesto.yaml at the project
// root.
type Config struct {
	// DefaultEnv is the environment used when none is named explicitly.
	DefaultEnv string `yaml:"default_env"`
	// EnvironmentsDir is the directory holding environment files, relative to the
	// project root. Defaults to "environments".
	EnvironmentsDir string `yaml:"environments_dir"`
}

// Environment is a single resolved environment (e.g. dev, staging). It is read
// from <environments_dir>/<name>/env.yaml, with the environment's static test
// data in <environments_dir>/<name>/data/*.yaml; or from a single
// <environments_dir>/<name>.yaml for an environment with no data.
type Environment struct {
	// Name is the environment name (the folder's or file's base name).
	Name string `yaml:"-"`
	// BaseURL is the API base URL that request paths resolve against.
	BaseURL string `yaml:"base_url"`
	// Auth configures authentication applied to every request.
	Auth *AuthConfig `yaml:"auth"`
	// Headers are default headers sent on every request.
	Headers map[string]string `yaml:"headers"`

	auth Auth    // resolved from Auth during loading
	data dataSet // the data/ folder, read with [T.Data]
}

// AuthConfig is the serialized form of an environment's authentication.
type AuthConfig struct {
	// Type is "bearer", "basic", or "none" (empty is treated as none).
	Type string `yaml:"type"`
	// Token is the bearer token (type: bearer).
	Token string `yaml:"token"`
	// Username and Password are the basic credentials (type: basic).
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// build turns an AuthConfig into an [Auth], validating required fields.
func (ac *AuthConfig) build() (Auth, error) {
	switch strings.ToLower(strings.TrimSpace(ac.Type)) {
	case "", "none":
		return nil, nil
	case "bearer":
		if ac.Token == "" {
			return nil, errors.New("bearer auth requires a token")
		}
		return BearerAuth(ac.Token), nil
	case "basic":
		return BasicAuth(ac.Username, ac.Password), nil
	default:
		return nil, fmt.Errorf("unknown auth type %q", ac.Type)
	}
}

// LoadConfig reads pesto.yaml from root. A missing file is not an error - it
// returns a zero Config, so a project can rely on environment files alone.
func LoadConfig(root string) (*Config, error) {
	b, err := os.ReadFile(filepath.Join(root, "pesto.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pesto.yaml: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse pesto.yaml: %w", err)
	}
	return &c, nil
}

// LoadEnvironment resolves an environment for a project rooted at root. When name
// is empty, the project's default_env from pesto.yaml is used. String values
// support ${VAR} / $VAR expansion from the process environment, so secrets like
// tokens stay out of the files.
func LoadEnvironment(root, name string) (*Environment, error) {
	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = cfg.DefaultEnv
	}
	if name == "" {
		return nil, errors.New("no environment specified and no default_env in pesto.yaml")
	}

	envDir := cfg.EnvironmentsDir
	if envDir == "" {
		envDir = defaultEnvironmentsDir
	}

	path, folder, err := environmentFile(root, envDir, name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read environment %q (%s): %w", name, path, err)
	}

	var env Environment
	if err := yaml.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	env.Name = name
	env.expandVars()

	if env.Auth != nil {
		a, err := env.Auth.build()
		if err != nil {
			return nil, fmt.Errorf("environment %q: %w", name, err)
		}
		env.auth = a
	}

	env.data = dataSet{files: map[string]any{}}
	if folder {
		display := filepath.Join(envDir, name, dataDirName)
		if env.data, err = loadData(filepath.Join(root, display), display); err != nil {
			return nil, fmt.Errorf("environment %q: %w", name, err)
		}
	}

	return &env, nil
}

// environmentFile finds the file defining environment name: the folder form
// <envDir>/<name>/env.yaml (folder is true) or the flat <envDir>/<name>.yaml.
// Defining an environment both ways is an error, so neither silently wins.
func environmentFile(root, envDir, name string) (path string, folder bool, err error) {
	flat := filepath.Join(root, envDir, name+".yaml")
	dir := filepath.Join(root, envDir, name)

	_, flatErr := os.Stat(flat)
	info, dirErr := os.Stat(dir)
	hasFlat, hasDir := flatErr == nil, dirErr == nil && info.IsDir()

	switch {
	case hasFlat && hasDir:
		return "", false, fmt.Errorf("environment %q is defined twice, as %s and %s%c: keep one", name, flat, dir, filepath.Separator)
	case hasDir:
		path := filepath.Join(dir, "env.yaml")
		if _, err := os.Stat(path); err != nil {
			return "", false, fmt.Errorf("environment %q: folder %s has no env.yaml", name, dir)
		}
		return path, true, nil
	case hasFlat:
		return flat, false, nil
	default:
		return "", false, fmt.Errorf("environment %q not found: want %s%cenv.yaml or %s", name, dir, filepath.Separator, flat)
	}
}

// expandVars applies ${VAR}/$VAR expansion to string fields.
func (e *Environment) expandVars() {
	e.BaseURL = os.ExpandEnv(e.BaseURL)
	if e.Auth != nil {
		e.Auth.Token = os.ExpandEnv(e.Auth.Token)
		e.Auth.Username = os.ExpandEnv(e.Auth.Username)
		e.Auth.Password = os.ExpandEnv(e.Auth.Password)
	}
	for k, v := range e.Headers {
		e.Headers[k] = os.ExpandEnv(v)
	}
}

// WithEnvironment configures a [T] from a resolved [Environment]: base URL,
// default headers, authentication, and static test data (read with [T.Data]).
func WithEnvironment(env *Environment) Option {
	return func(c *config) {
		if env.BaseURL != "" {
			c.baseURL = env.BaseURL
		}
		for k, v := range env.Headers {
			c.headers.Set(k, v)
		}
		if env.auth != nil {
			c.auth = env.auth
		}
		if env.data.dir != "" {
			c.data.dir = env.data.dir
		}
		for k, v := range env.data.files {
			c.data.files[k] = v
		}
	}
}
