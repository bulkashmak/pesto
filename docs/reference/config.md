# Configuration reference

Pesto reads two kinds of files from the project root: `pesto.yaml` (project
settings) and `environments/<name>.yaml` (per-environment settings). This reflects
what is implemented today.

## `pesto.yaml`

```yaml
# The environment used by `pesto run` when --env is not given.
default_env: dev

# Directory holding environment files, relative to the root. Optional.
environments_dir: environments
```

| Key | Default | Meaning |
| --- | --- | --- |
| `default_env` | (none) | environment selected when `--env` is absent |
| `environments_dir` | `environments` | where environment files live |

`pesto.yaml` is optional: a project can rely on `--env` plus environment files
alone. If neither `--env` nor `default_env` selects an environment, `pesto run`
runs with no environment (tests must then configure their client explicitly).

## `environments/<name>.yaml`

```yaml
# base_url is where t.Client sends requests; test paths are relative to it.
base_url: https://api.staging.acme.example

auth:
  type: bearer
  token: ${ACME_TOKEN}

headers:
  X-Team: qa

variables:
  default_account: "42"
```

| Key | Meaning |
| --- | --- |
| `base_url` | base URL prepended to request paths |
| `auth` | authentication applied to every request (see below) |
| `headers` | default headers sent on every request |
| `variables` | free-form string values a test can reference |

### Authentication

```yaml
auth:
  type: bearer         # bearer | basic | none
  token: ${ACME_TOKEN} # for bearer
  # username: alice    # for basic
  # password: ${PW}    # for basic
```

- `bearer` sends `Authorization: Bearer <token>` (a token is required).
- `basic` sends HTTP Basic credentials.
- `none` (or omitting `auth`) applies no authentication.

A request that sets `Authorization` explicitly overrides the environment auth.

### Variable expansion

Every string value supports `${VAR}` / `$VAR` expansion from the process
environment, evaluated when the environment is loaded. Keep secrets (tokens,
passwords) out of the files and inject them via the environment:

```bash
ACME_TOKEN=$(vault read ...) pesto run --env staging
```

## Using config without the CLI

The runtime library is usable from a plain `func TestX(t *testing.T)` (the escape
hatch, e.g. for dogfooding). Options mirror the file format:

```go
env, _ := pesto.LoadEnvironment(".", "dev")
pt := pesto.New(t, pesto.WithEnvironment(env))

// or configure directly:
pt := pesto.New(t,
	pesto.WithBaseURL("http://localhost:8080"),
	pesto.WithAuth(pesto.BearerAuth("token")),
	pesto.WithHeader("X-Team", "qa"),
)
```

Under `pesto run`, this wiring is automatic: the selected environment reaches each
test through `pesto.Run`.
