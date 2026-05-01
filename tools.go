//go:build tools

// Package tools pins tool and future-phase dependencies so that
// `go mod tidy` does not remove them before they are imported in
// production code. Remove individual imports as each refactoring
// phase adds real usage.
package tools

import (
	_ "github.com/Masterminds/squirrel"
	_ "github.com/jackc/pgx/v5"
	_ "go.uber.org/mock/mockgen"
)
