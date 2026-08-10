# Contributing to gomemory

Thanks for your interest in contributing to gomemory.

## Getting Started

1. Fork the repository
2. Clone your fork
3. Create a feature branch: `git checkout -b my-feature`
4. Make your changes
5. Run the tests: `go test ./...`
6. Run the linter: `go vet ./...`
7. Commit your changes
8. Push to your fork and open a Pull Request

## Development Requirements

- Go 1.25+
- No CGO required (SQLite is pure Go via `modernc.org/sqlite`)

## Building

```bash
go build -o mem ./infrastructure/
```

## Running Tests

```bash
go test ./...
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep changes minimal and focused
- One concern per pull request

## Pull Request Guidelines

- Describe what changed and why
- Reference any related issues
- Ensure `go test ./...` and `go vet ./...` pass
- Keep PRs small and reviewable

## Reporting Bugs

Open an issue with:
- Steps to reproduce
- Expected vs actual behavior
- Go version and OS

## Feature Requests

Open an issue describing the use case and proposed solution before implementing.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
