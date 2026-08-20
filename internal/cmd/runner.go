// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// TextFormatter is a function that formats data in human-readable text mode.
type TextFormatter[T any] func(w io.Writer, data T) error

// HandledError indicates that an error was already formatted and output to the user.
type HandledError struct {
	Err error
}

func (e *HandledError) Error() string {
	return e.Err.Error()
}

func (e *HandledError) Unwrap() error {
	return e.Err
}

// GetFormat returns the output format configured for the command ("text" or "json").
func GetFormat(cmd *cobra.Command) string {
	if cmd != nil {
		if format, err := cmd.Flags().GetString("format"); err == nil && format != "" {
			return format
		}
	}
	if outputFormat != "" {
		return outputFormat
	}
	return "text"
}

// RunHandler runs a subcommand handler function, handling output formatting and stream routing.
func RunHandler[T any](cmd *cobra.Command, textFormatter TextFormatter[T], fn func(ctx context.Context) (T, error)) error {
	format := GetFormat(cmd)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	data, err := fn(ctx)
	if err != nil {
		if format == "json" {
			if writeErr := WriteJSONError(cmd.OutOrStdout(), err); writeErr != nil {
				return writeErr
			}
			return &HandledError{Err: err}
		}
		// Text mode: return error directly so it gets routed to stderr
		return err
	}

	if format == "json" {
		return WriteJSONSuccess(cmd.OutOrStdout(), data)
	}

	if textFormatter != nil {
		return textFormatter(cmd.OutOrStdout(), data)
	}

	return nil
}

// ExecuteRoot executes a root command and intercepts errors to format them consistently.
func ExecuteRoot(cmd *cobra.Command) error {
	err := cmd.Execute()
	if err != nil {
		handleExecutionError(cmd, err)
		return err
	}
	return nil
}

func handleExecutionError(cmd *cobra.Command, err error) {
	var handled *HandledError
	if errors.As(err, &handled) {
		// Error was already formatted by RunHandler
		return
	}

	if GetFormat(cmd) == "json" {
		_ = WriteJSONError(cmd.OutOrStdout(), err)
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
	}
}
