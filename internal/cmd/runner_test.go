// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type dummyData struct {
	Message string `json:"message"`
}

func dummyTextFormatter(w io.Writer, data dummyData) error {
	_, err := fmt.Fprintf(w, "Message: %s\n", data.Message)
	return err
}

func TestRunHandler_JSON_Success(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "json", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := RunHandler(cmd, dummyTextFormatter, func(ctx context.Context) (dummyData, error) {
		return dummyData{Message: "hello world"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stderr.Len() > 0 {
		t.Errorf("expected empty stderr, got %q", stderr.String())
	}

	var env ResponseEnvelope[dummyData]
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON from stdout: %v, raw: %q", err, stdout.String())
	}

	if env.Status != "success" {
		t.Errorf("expected status 'success', got %q", env.Status)
	}
	if env.Data.Message != "hello world" {
		t.Errorf("expected message 'hello world', got %q", env.Data.Message)
	}
}

func TestRunHandler_JSON_Error(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "json", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	expectedErr := errors.New("backend failed")
	err := RunHandler(cmd, dummyTextFormatter, func(ctx context.Context) (dummyData, error) {
		return dummyData{}, expectedErr
	})

	if err == nil {
		t.Fatal("expected error from RunHandler, got nil")
	}

	var handled *HandledError
	if !errors.As(err, &handled) {
		t.Errorf("expected error to be *HandledError, got %T: %v", err, err)
	}

	if stderr.Len() > 0 {
		t.Errorf("expected empty stderr in JSON mode, got %q", stderr.String())
	}

	var env ErrorEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON error from stdout: %v, raw: %q", err, stdout.String())
	}

	if env.Status != "error" {
		t.Errorf("expected status 'error', got %q", env.Status)
	}
	if env.Error != "backend failed" {
		t.Errorf("expected error 'backend failed', got %q", env.Error)
	}
}

func TestRunHandler_Text_Success(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "text", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := RunHandler(cmd, dummyTextFormatter, func(ctx context.Context) (dummyData, error) {
		return dummyData{Message: "hello world"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stderr.Len() > 0 {
		t.Errorf("expected empty stderr, got %q", stderr.String())
	}

	if !strings.Contains(stdout.String(), "Message: hello world") {
		t.Errorf("expected text output on stdout, got %q", stdout.String())
	}
}

func TestRunHandler_Text_Error(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("format", "text", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	expectedErr := errors.New("backend failed")
	err := RunHandler(cmd, dummyTextFormatter, func(ctx context.Context) (dummyData, error) {
		return dummyData{}, expectedErr
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if errors.Is(err, expectedErr) == false {
		t.Errorf("expected returned error to be expectedErr, got %v", err)
	}

	if stdout.Len() > 0 {
		t.Errorf("expected empty stdout in text error, got %q", stdout.String())
	}
}

func TestExecuteRoot_FlagError_JSON(t *testing.T) {
	root := &cobra.Command{
		Use:           "root",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var fmtFlag string
	root.PersistentFlags().StringVar(&fmtFlag, "format", "text", "")

	sub := &cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	var reqFlag string
	sub.Flags().StringVar(&reqFlag, "required", "", "")
	_ = sub.MarkFlagRequired("required")
	root.AddCommand(sub)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"sub", "--format", "json"})

	err := ExecuteRoot(root)
	if err == nil {
		t.Fatal("expected flag error, got nil")
	}

	var env ErrorEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON error from stdout: %v, raw: %q", err, stdout.String())
	}

	if env.Status != "error" {
		t.Errorf("expected status 'error', got %q", env.Status)
	}
	if !strings.Contains(env.Error, "required flag") {
		t.Errorf("expected error to mention required flag, got %q", env.Error)
	}
}

func TestExecuteRoot_FlagError_Text(t *testing.T) {
	root := &cobra.Command{
		Use:           "root",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	var fmtFlag string
	root.PersistentFlags().StringVar(&fmtFlag, "format", "text", "")

	sub := &cobra.Command{
		Use: "sub",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	var reqFlag string
	sub.Flags().StringVar(&reqFlag, "required", "", "")
	_ = sub.MarkFlagRequired("required")
	root.AddCommand(sub)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"sub", "--format", "text"})

	err := ExecuteRoot(root)
	if err == nil {
		t.Fatal("expected flag error, got nil")
	}

	if stdout.Len() > 0 {
		t.Errorf("expected empty stdout on text flag error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: ") {
		t.Errorf("expected stderr to contain 'error: ' prefix, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "required flag") {
		t.Errorf("expected error on stderr, got %q", stderr.String())
	}
}
