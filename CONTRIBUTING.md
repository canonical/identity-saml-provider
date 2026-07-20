# Contributing

## Pre-commit Hooks

This repository uses [pre-commit](https://pre-commit.com/)
to run linters and checks automatically before each commit.

### Installation

Install the pre-commit tool, then set up both the
`pre-commit` and `commit-msg` hooks:

```shell
pip install pre-commit
pre-commit install -t pre-commit -t commit-msg
```

### Code Generation (mockgen)

This project uses
[`go.uber.org/mock/mockgen`](https://github.com/uber-go/mock)
to generate mock implementations from interfaces. Install
it once:

```shell
go install go.uber.org/mock/mockgen@latest
```

After adding or modifying `//go:generate` directives, run:

```shell
make generate
```

This invokes `go generate ./...` and refreshes all generated
files under `mocks/`.

### Running Hooks Manually

To run all hooks against every file in the repository:

```shell
pre-commit run --all-files
```

To run a specific hook:

```shell
pre-commit run <hook-id> --all-files
```

## Commits

When contributing code, please follow the
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
specification for commit messages. This is enforced by
the `conventional-pre-commit` hook at the `commit-msg`
stage.

## Code Verification Suite

Before submitting a pull request, ensure the codebase compiles and satisfies
all quality and style checks:

```shell
make fmt            # Format Go source code files
make lint           # Execute golangci-lint
make test           # Run unit tests
make license-check  # Verify license headers
make build          # Build the production executable
```
