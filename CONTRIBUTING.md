# Contributing to Skriva

Thank you for your interest in contributing! This guide covers the development workflow, coding standards, and review process.

## Development Environment

### Prerequisites

**All Go tooling runs inside the dev container — no local Go installation required.**

1. Install [VS Code](https://code.visualstudio.com/) and the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
2. Clone the repository and open it in VS Code
3. When prompted, click **"Reopen in Container"** — the `.devcontainer/devcontainer.json` will set up Go 1.25+ and all required tools

### First Run

```bash
go mod tidy          # Generate go.sum
go build ./cmd/blog  # Verify the binary builds
go test -race ./...  # Run all tests with race detector
```

## Code Structure

The project follows a clean internal package layout. See the [Architecture section](README.md) in the README for full details.

| Directory            | Purpose                                          |
| -------------------- | ------------------------------------------------ |
| `cmd/blog/`          | CLI entry point (`main.go`)                      |
| `internal/handler/`  | HTTP handlers (public, admin API, auth, etc.)    |
| `internal/store/`    | SQLite database (queries, migrations)            |
| `internal/render/`   | Markdown pipeline, template rendering, cache     |
| `internal/content/`  | Filesystem post/page loading, hot-reload watcher |
| `internal/config/`   | YAML config loading, validation                  |
| `internal/server/`   | HTTP server setup, routes, middleware            |
| `internal/plugin/`   | Hook-based plugin system                         |
| `internal/importer/` | Ghost and WordPress importers                    |
| `themes/`            | Bundled theme files (embedded into binary)       |

## Coding Standards

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- All code must pass `golangci-lint run ./...` (config: [.golangci.yml](.golangci.yml))
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Use `context.Context` on all I/O operations
- Use `log/slog` for structured logging
- No global mutable state — pass dependencies via struct fields

## Running Tests

```bash
# All tests with race detector
go test -race ./...

# With coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

# Specific package
go test -race ./internal/handler/...

# Benchmarks
go test -bench=. ./internal/render/...

# Fuzz tests (30 seconds each)
go test -fuzz=FuzzSanitizeUntrustedHTML -fuzztime=30s ./internal/handler/
go test -fuzz=FuzzSplitFrontmatter -fuzztime=30s ./internal/content/
```

## Linting

```bash
# Install golangci-lint v2
# See: https://golangci-lint.run/welcome/install/

# Run linter
golangci-lint run ./...

# Auto-fix where possible
golangci-lint run --fix ./...
```

## Security Checks (Mandatory)

These checks must pass before any PR can be merged:

```bash
go build ./cmd/blog     # Must compile with zero errors
go vet ./...            # Must pass with zero warnings
golangci-lint run ./... # Must pass with zero issues
go test -race ./...     # Must pass with no data race warnings
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with clear, atomic commits
3. Ensure all [security checks](#security-checks-mandatory) pass locally
4. Open a PR with a description of what changed and why
5. Wait for CI to pass (test, vet, lint, govulncheck, Docker build)

## Reporting Security Issues

**Do not open a public issue for security vulnerabilities.**

See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
