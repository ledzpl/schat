# Repository Guidelines

## Project Structure & Module Organization
- Keep the Go entrypoint under `cmd/schat/main.go`; wire SSH session handling from here so the CLI stays minimal.
- Place conversational logic, user registry, and message broadcasting under `internal/chat/...` so dependencies remain private to the module.
- Share reusable helpers (e.g., SSH auth, configuration loaders) through `pkg/` packages.
- Store integration test fixtures in `testdata/` and configuration templates in `configs/`.
- Document runbooks or design notes in `docs/` to keep operational knowledge adjacent to code.

## Build, Test, and Development Commands
- `go mod tidy` keeps module metadata clean after adding dependencies.
- `go build ./cmd/schat` verifies the server builds; use `GOOS`/`GOARCH` flags for cross-compilation.
- `go run ./cmd/schat --config configs/dev.yaml` starts a local SSH chat node with the dev profile.
- `go test ./...` executes all unit tests; add `-run <Regex>` when focusing on one suite.
- `golangci-lint run` enforces formatting, vetting, and static checks; update `.golangci.yml` when enabling new linters.

## Coding Style & Naming Conventions
- Format code with `gofmt` (tabs) and `goimports`; never commit unformatted files.
- Use descriptive package names (e.g., `session`, `transport`) and CamelCase exported identifiers.
- Interface names describe capability (`Broadcaster`, `Authenticator`), while structs end with their role (`Service`, `Repository`).

## Testing Guidelines
- Unit tests belong alongside sources as `*_test.go` and use table-driven cases for clarity.
- Favor `testing` plus `testify/require` for assertions; keep integration tests under `./test`.
- Aim for ≥80% coverage on core chat flows; add regression tests for every bug fix.
- Run `go test -race ./...` before merging to catch data races in concurrent code.

## Commit & Pull Request Guidelines
- Follow Conventional Commits (`feat:`, `fix:`, `chore:`) with imperative subject lines ≤72 chars.
- Reference tickets or SPEC items in the body, and describe behavior changes plus verification steps.
- Pull requests should link context, include logs or terminal transcripts when touching SSH handling, and outline manual validation.
- Request review from another maintainer; wait for CI green before merging.
