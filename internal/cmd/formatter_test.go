// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

type testPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSONSuccess(t *testing.T) {
	tests := []struct {
		name     string
		payload  any
		validate func(t *testing.T, raw []byte)
	}{
		{
			name: "struct payload",
			payload: testPayload{
				Name:  "test-item",
				Count: 5,
			},
			validate: func(t *testing.T, raw []byte) {
				var env ResponseEnvelope[testPayload]
				if err := json.Unmarshal(raw, &env); err != nil {
					t.Fatalf("failed to unmarshal envelope: %v", err)
				}
				if env.Status != "success" {
					t.Errorf("expected status 'success', got %q", env.Status)
				}
				if env.Data.Name != "test-item" || env.Data.Count != 5 {
					t.Errorf("unexpected data: %+v", env.Data)
				}
				if env.Error != "" {
					t.Errorf("expected empty error, got %q", env.Error)
				}
			},
		},
		{
			name:    "slice payload",
			payload: []string{"first", "second"},
			validate: func(t *testing.T, raw []byte) {
				var env ResponseEnvelope[[]string]
				if err := json.Unmarshal(raw, &env); err != nil {
					t.Fatalf("failed to unmarshal envelope: %v", err)
				}
				if env.Status != "success" {
					t.Errorf("expected status 'success', got %q", env.Status)
				}
				if len(env.Data) != 2 || env.Data[0] != "first" || env.Data[1] != "second" {
					t.Errorf("unexpected data: %+v", env.Data)
				}
			},
		},
		{
			name:    "primitive payload",
			payload: int64(12345),
			validate: func(t *testing.T, raw []byte) {
				var env ResponseEnvelope[int64]
				if err := json.Unmarshal(raw, &env); err != nil {
					t.Fatalf("failed to unmarshal envelope: %v", err)
				}
				if env.Status != "success" {
					t.Errorf("expected status 'success', got %q", env.Status)
				}
				if env.Data != 12345 {
					t.Errorf("expected data 12345, got %d", env.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteJSONSuccess(&buf, tt.payload); err != nil {
				t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
			}
			tt.validate(t, buf.Bytes())
		})
	}
}

func TestWriteJSONError(t *testing.T) {
	errToEncode := errors.New("something went wrong")
	var buf bytes.Buffer

	if err := WriteJSONError(&buf, errToEncode); err != nil {
		t.Fatalf("WriteJSONError() unexpected error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if raw["status"] != "error" {
		t.Errorf("expected status 'error', got %v", raw["status"])
	}
	if raw["error"] != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %v", raw["error"])
	}
	if _, ok := raw["data"]; ok {
		t.Errorf("expected no 'data' field in error response, got %v", raw["data"])
	}
}
