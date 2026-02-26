# Repository Guidelines

## Project Structure & Module Organization
This repo is a Go multi-module monorepo for Yggdrasil ecosystem integrations.
- `integrations/{etcd,k8s,otlp,polaris,xds}`: primary integration libraries (each has its own `go.mod`).
- `integrations/*/example`: runnable sample apps and configs for each integration.
- `examples/protogen`: generated protobuf artifacts used for demos and compatibility checks.
- `scripts/make-rules`: shared Make targets; `scripts/hooks`: commit/branch policy checks.

## Build, Test, and Development Commands
Use the root `Makefile` for all routine workflows:
- `make tools`: install required tools and pre-commit hooks.
- `make sync`: generate/update `go.work` from discovered modules.
- `make tidy`: run `go mod tidy` across all modules.
- `make fmt && make lint`: format and lint all modules.
- `make test` / `make coverage`: run tests (coverage gate defaults to `COVERAGE=60`).
- `make test INCLUDE_EXAMPLES=1`: include example modules in test runs.

## Coding Style & Naming Conventions
- Go formatting is enforced via `make fmt` (`gofumpt`, `goimports`, `golines --max-len=100`).
- Keep package names lowercase and concise; avoid underscores in package names.
- Exported Go identifiers should include GoDoc comments (enforced by `revive`).
- Keep module APIs in integration roots; place demo-only code under `example/`.
- Shell scripts should be Bash with strict mode (`errexit`, `nounset`, `pipefail`).

## Testing Guidelines
- Place tests alongside code in `*_test.go` files.
- Use `testing` package conventions: `TestXxx` names and table-driven `t.Run(...)` subtests where useful.
- Run `make test` before pushing; run `make coverage` for behavior-heavy changes.
- Add or update example tests only when example behavior or docs change.

## Commit & Pull Request Guidelines
- Commit messages follow Conventional Commits (validated by gitlint), e.g. `feat(xds): add route matcher fallback`.
- Subject line max 72 chars; body lines max 100 chars; keep subject lowercase after `: `.
- Breaking changes must use `!` and include a `BREAKING CHANGE:` footer in the body.
- Branch names must match: `feature/<name>`, `release/<name>`, `hotfix/<name>`, or `main|master|develop`.
- PRs should include: what changed, affected module paths, commands run (lint/test/coverage), and config/example updates when relevant.
