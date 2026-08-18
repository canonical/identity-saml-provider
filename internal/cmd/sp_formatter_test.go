// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatSPRegistered(t *testing.T) {
	sp := &SPResult{
		EntityID:   "http://example.com/metadata",
		ACSURL:     "http://example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}

	var buf bytes.Buffer
	if err := formatSPRegistered(&buf, sp); err != nil {
		t.Fatalf("formatSPRegistered() unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"✓ Service provider registered successfully!",
		"http://example.com/metadata",
		"http://example.com/acs",
		"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got %q", want, output)
		}
	}
}

func TestSPResultJSONEnvelope(t *testing.T) {
	sp := &SPResult{
		EntityID:   "http://example.com/metadata",
		ACSURL:     "http://example.com/acs",
		ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	}

	var buf bytes.Buffer
	if err := WriteJSONSuccess(&buf, sp); err != nil {
		t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
	}

	var env ResponseEnvelope[SPResult]
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if env.Status != "success" {
		t.Errorf("expected status 'success', got %q", env.Status)
	}
	if env.Data.EntityID != sp.EntityID {
		t.Errorf("expected entity_id %q, got %q", sp.EntityID, env.Data.EntityID)
	}
	if env.Data.ACSURL != sp.ACSURL {
		t.Errorf("expected acs_url %q, got %q", sp.ACSURL, env.Data.ACSURL)
	}
	if env.Data.ACSBinding != sp.ACSBinding {
		t.Errorf("expected acs_binding %q, got %q", sp.ACSBinding, env.Data.ACSBinding)
	}
}
