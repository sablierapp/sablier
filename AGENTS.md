## Project

Sablier starts workloads on demand and stops them when they are idle. The code is in Go. The documentation site is in `docs`.

## Communication

- Use ASD-STE100 Simplified Technical English when you speak to the operator.
- Use ASD-STE100 for all text outside of the code: commit messages, PR titles, PR descriptions, issues, and GitHub comments.

## Line wrapping

- Do not wrap lines when the consuming system adapts the view. Examples: Markdown files, PR descriptions, issues, and GitHub comments.
- Wrap the body of a commit message at 72 characters.

## Code comments

- Do not write redundant comments. If a comment is necessary, keep it short. Use no more than 2 lines.
- Use few colons, semicolons, and dashes in comments.
- If a comment explains a specific requirement, add a reference to the source. Examples: a datasheet, a specification, or a URL. Include the chapter, section, or page number.

These rules do not apply to structured comments. Tools parse these comments, or the comments have a fixed format. Do not shorten or remove them:

- The metadata lines (`type:`, `default:`, `example:`, `providers:`, and others) on the label constants in `pkg/sablier/labels.go`. `cmd/labelsgen` parses them.
- The swag annotations (`// @Summary`, `// @Router`, and others) in `internal/api`. `cmd/openapigen` parses them.
- The `Env:`, `CLI:`, and `Default:` lines on the configuration fields in `pkg/config`.

## Repository map

| Path | Content |
|---|---|
| `cmd/sablier` | The main binary. |
| `cmd/docgen`, `cmd/labelsgen`, `cmd/metricsgen`, `cmd/openapigen`, `cmd/schemagen` | The generators for the files in `docs`. |
| `pkg/sablier` | The core logic: sessions, instances, labels, and the `Provider` and `Store` interfaces. |
| `pkg/provider/<name>` | One package for each provider. `internal` and `providertest` are not providers. |
| `pkg/sabliercmd` | The CLI commands, the flags, and the provider setup. |
| `pkg/config` | The configuration structs. |
| `pkg/theme` | The waiting page themes. The built-in themes are in `pkg/theme/embedded`. |
| `pkg/store` | The session stores: `inmemory` and `valkey`. |
| `internal/api`, `internal/server` | The HTTP handlers and the routes. |
| `docs` | The Hugo documentation site. |
| `examples` | Example setups for features and providers. |

## Tools

- `.tool-versions` sets the versions of Go, golangci-lint, goreleaser, and Hugo. Run `mise install` to install them.
- `tools.mod` has the development tools. Run a tool with `go tool -modfile=tools.mod <tool>`. Do not add development tools to `go.mod`.
- Use LF line endings. `.gitattributes` sets this.

## Commands

| Command | Purpose |
|---|---|
| `make fmt` | Format the code. |
| `make lint` | Run golangci-lint. |
| `make test` | Run all tests. Some tests need Docker. |
| `go test -short ./...` | Run only the tests that do not need Docker. |
| `make generate` | Generate the reference docs, the OpenAPI spec, and the theme schema. |
| `make gen` | Run `go generate ./...`. This also generates the mocks. |
| `make check-generate` | Make sure that the generated files are up to date. CI runs this check. |
| `make run` | Start Sablier locally with debug logs. |
| `make docs` | Start the docs site locally. |

Before you tell the operator that a task is complete, run `make fmt`, `make lint`, `make check-generate`, and the tests of the changed packages. When you change a provider, also run the tests of that provider without `-short`. CI runs all tests with `-tags=nomsgpack -race`.

## Generated files

Do not edit these files by hand. Change the source, then run the command.

| File | Source | Command |
|---|---|---|
| `docs/content/reference/cli.md` | The flags in `pkg/sabliercmd` | `make cli-docs` |
| `docs/content/reference/labels.md` | The comments in `pkg/sablier/labels.go` | `make labels-docs` |
| `docs/content/how-to-guides/advanced/observability/metrics.md` | The collectors in `pkg/metrics` | `make metrics-docs` |
| `docs/static/openapi.json` | The swag annotations in `internal/api` | `make openapi` |
| `docs/static/theme.schema.json` | The types in `pkg/theme/types.go` | `make schema` |
| `pkg/provider/providertest/mock_provider.go` | `pkg/sablier/provider.go` | `make gen` |
| `pkg/store/storetest/mocks_store.go` | `pkg/sablier/store.go` | `make gen` |
| `internal/api/apitest/mocks_sablier.go` | `internal/api/api.go` | `make gen` |

release-please writes `CHANGELOG.md` and the version numbers near the `x-release-please` markers in `README.md` and `docs/hugo.yaml`. Do not change them.

## Tests

- Write new tests with the standard library only. Examples: `testing`, `errors`, `reflect`, `slices`, and `maps`.
- Do not add `gotest.tools`, `testify`, `go-cmp`, or other assertion libraries to a test. Some old tests still use these libraries. Do not migrate the old tests unless the task asks for it.
- You can use the generated mocks (`go.uber.org/mock`) and Testcontainers.
- If a test needs Docker or a real cluster, call `t.Skip` when `testing.Short()` is true.
- If possible, use a fake that runs in short mode. Example: the fake clientset for Kubernetes.
- When you fix a bug, add a test that fails without the fix.

## Providers

- Each folder in `pkg/provider` is a provider, except `internal` and `providertest`.
- The `Provider` interface is in `pkg/sablier/provider.go`. If you change it, update all providers and run `make gen`.
- If you fix a bug in one provider, look for the same bug in the other providers.
- If a feature does not work on all providers, write the limits in the `providers:` line of the label comment and in the docs.

## Checklists

### New configuration option

1. Add the field and its `Env:`, `CLI:`, and `Default:` lines in `pkg/config`.
2. Add the flag and its viper binding in `pkg/sabliercmd/root.go`.
3. Update the files in `pkg/sabliercmd/testdata` and `pkg/sabliercmd/cmd_test.go`.
4. Run `make cli-docs`.

### New label

1. Declare the constant and its structured comment in `pkg/sablier/labels.go`. A test fails if the code uses a label that is not declared there.
2. Run `make labels-docs`.

### New API endpoint

1. Add the handler, its swag annotations, and its test in `internal/api`.
2. Add the route in `internal/server/routes.go`.
3. Run `make openapi`.
4. Update the docs in `docs/content`.

### New built-in theme

1. Add `pkg/theme/embedded/<name>.html`.
2. Add the theme name to the expected lists in `pkg/theme/list_test.go` and `pkg/sabliercmd/theme_test.go`.
3. Add a render test in `pkg/theme/render_test.go`.
4. Add the screenshot `docs/static/assets/img/<name>.png`.
5. Add the theme to the table and to the JSON list in `docs/content/how-to-guides/loading-strategies/customize-theme.md`.
6. Add the theme to the list in `docs/content/tutorials/getting-started.md`.

### New provider

1. Add the package `pkg/provider/<name>`.
2. Add the configuration in `pkg/config/provider.go`, the flags in `pkg/sabliercmd/root.go`, and the setup in `pkg/sabliercmd/provider.go`.
3. Update the files in `pkg/sabliercmd/testdata` and `pkg/sabliercmd/cmd_test.go`.
4. Add the provider to `docs/data/providers.yaml`. Add its icon in `docs/static/assets/img`. Add a page in `docs/content/tutorials/providers`.
5. Add the provider to `README.md`.
6. Run `make generate`.

## Documentation

- The docs site uses Hugo and the Hextra theme.
- Put each page in the correct section of `docs/content`: `tutorials`, `how-to-guides`, `concepts`, `reference`, or `guides`.
- If users can see a change, update the docs in the same PR.

## Commits and pull requests

- Use Conventional Commits: `<type>(<scope>): <description>`. `CONTRIBUTING.md` lists the types.
- Use the package or provider name as the scope. Examples: `kubernetes`, `docker`, `api`, `theme`, and `sablier`. If the change has no single area, do not use a scope.
- The PR title becomes the commit title on `main`. release-please reads it to calculate the version and to write the changelog. The `build` and `ci` types do not appear in the changelog.
- Do not use `chore(main)`. release-please uses it for release PRs.
- Use `build(deps)` only for dependency updates.
- In the commit body, tell what was wrong and why the change is correct. If the change closes an issue, add `Fixes #<number>`.
- If an AI agent helped with the change, add a `Co-authored-by:` trailer for the agent.
- Do not commit secrets or the `state.json` file from `make run`.
