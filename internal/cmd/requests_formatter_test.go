// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatRequestsPrune(t *testing.T) {
	var buf bytes.Buffer
	res := RequestsPruneResult{DeletedCount: 42}
	if err := formatRequestsPrune(&buf, res); err != nil {
		t.Fatalf("formatRequestsPrune() unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[Pruned] successfully deleted 42 expired pending request(s).") {
		t.Errorf("expected pruned text format, got %q", output)
	}
}

func TestRequestsPruneResultJSONEnvelope(t *testing.T) {
	var buf bytes.Buffer
	res := RequestsPruneResult{DeletedCount: 100}
	if err := WriteJSONSuccess(&buf, res); err != nil {
		t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
	}

	var env ResponseEnvelope[RequestsPruneResult]
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if env.Status != "success" {
		t.Errorf("expected status 'success', got %q", env.Status)
	}
	if env.Data.DeletedCount != 100 {
		t.Errorf("expected deleted_count 100, got %d", env.Data.DeletedCount)
	}
}
