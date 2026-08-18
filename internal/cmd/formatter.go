// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"encoding/json"
	"io"
)

// ResponseEnvelope is the standard envelope for structured CLI JSON responses.
type ResponseEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
	Error  string `json:"error,omitempty"`
}

// ErrorEnvelope is the standard envelope for error CLI JSON responses.
type ErrorEnvelope struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// WriteJSONSuccess writes a standardized success JSON envelope to w.
func WriteJSONSuccess[T any](w io.Writer, data T) error {
	return json.NewEncoder(w).Encode(ResponseEnvelope[T]{
		Status: "success",
		Data:   data,
	})
}

// WriteJSONError writes a standardized error JSON envelope to w.
func WriteJSONError(w io.Writer, err error) error {
	return json.NewEncoder(w).Encode(ErrorEnvelope{
		Status: "error",
		Error:  err.Error(),
	})
}
