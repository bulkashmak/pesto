# Configuration reference

Pesto reads `pesto.yaml` (project settings) from the project root, and the
selected environment from `environments/<name>/` (connection settings plus static
test data). This reflects what is implemented today.

## `pesto.yaml`

```yaml
# The environment used by `pesto run` when --env is not given.
default_env: dev

# Directory holding environments, relative to the root. Optional.
environments_dir: environments
```

| Key | Default | Meaning |
| --- | --- | --- |
| `default_env` | (none) | environment selected when `--env` is absent |
| `environments_dir` | `environments` | where environments live |

`pesto.yaml` is optional: a project can rely on `--env` plus environment files
alone. If neither `--env` nor `default_env` selects an environment, `pesto run`
runs with no environment (tests must then configure their client explicitly).

## Environments

An environment is a folder:

```
environments/
└── staging/
    ├── env.yaml        # where and how to connect
    └── data/           # static test data - optional
        ├── users.yaml
        └── cards.yaml
```

An environment with no data may instead be a single file,
`environments/<name>.yaml`, with the same keys as `env.yaml`. Defining an
environment both ways is an error.

### `env.yaml`

```yaml
# base_url is where t.Client sends requests; test paths are relative to it.
base_url: https://api.staging.acme.example

auth:
  type: bearer
  token: ${ACME_TOKEN}

headers:
  X-Team: qa
```

| Key | Meaning |
| --- | --- |
| `base_url` | base URL prepended to request paths |
| `auth` | authentication applied to every request (see below) |
| `headers` | default headers sent on every request |

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

## Static test data

Static test data is predefined data that is permanent in the environment's
database - tests only read it. Each `data/<file>.yaml` is one entity; its
top-level keys are named records, and each record is a map of fields:

```yaml
# environments/staging/data/users.yaml
default:
  id: 9001
  email: qa-default@acme.example
  password: ${STAGING_DEFAULT_PASSWORD}
  address:
    city: Kazan
blocked:
  id: 9002
```

- Values are scalars (strings, numbers, booleans) or nested maps of fields.
  Lists are not supported.
- Values support the same `${VAR}` expansion as `env.yaml`.
- File, record and field names cannot contain a dot. Only `.yaml` files are read.

A **reference** names one value: `<file>.<record>.<field>[.<nested>...]`.

| Reference | Value |
| --- | --- |
| `users.default.id` | `9001` |
| `users.default.address.city` | `Kazan` |

Read it in a test with `t.Data(ref)`, which returns the value as a string in its
YAML form (`"9001"`). A missing reference fails the test immediately:

```
pesto: data users.default.id: no record "default" in environments/staging/data/users.yaml
```

`t.LookupData(ref)` returns `(value, ok)` instead, for optional data. Contract
tests use references in [`path_params`](contract.md#contractyaml).

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
	pesto.WithData(map[string]any{
		"users": map[string]any{"default": map[string]any{"id": 42}},
	}),
)
```

Under `pesto run`, this wiring is automatic: the selected environment reaches each
test through `pesto.Run`.
