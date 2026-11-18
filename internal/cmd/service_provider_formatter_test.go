// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/canonical/identity-saml-provider/internal/domain"
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

func TestFormatSPList(t *testing.T) {
	sps := []*SPListItem{
		{
			EntityID:   "http://a.example.com/metadata",
			ACSURL:     "http://a.example.com/acs",
			ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		},
		{
			EntityID:   "http://b.example.com/metadata",
			ACSURL:     "http://b.example.com/acs",
			ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		},
	}

	var buf bytes.Buffer
	if err := formatSPList(&buf, sps); err != nil {
		t.Fatalf("formatSPList() unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"http://a.example.com/metadata",
		"http://a.example.com/acs",
		"http://b.example.com/metadata",
		"http://b.example.com/acs",
		"urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got %q", want, output)
		}
	}
}

func TestFormatSPListEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := formatSPList(&buf, []*SPListItem{}); err != nil {
		t.Fatalf("formatSPList() unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No service providers registered.") {
		t.Errorf("expected friendly empty message, got %q", buf.String())
	}
}

func TestSPListJSONEnvelope(t *testing.T) {
	sps := []*SPListItem{
		{
			EntityID:   "http://a.example.com/metadata",
			ACSURL:     "http://a.example.com/acs",
			ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
			AttributeMapping: &domain.AttributeMapping{
				NameIDFormat: "persistent",
				SAMLAttributeMappings: map[string]domain.SAMLAttributeDef{
					"email": {Name: "mail", FriendlyName: "mail"},
				},
			},
		},
		{
			EntityID:   "http://b.example.com/metadata",
			ACSURL:     "http://b.example.com/acs",
			ACSBinding: "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		},
	}

	var buf bytes.Buffer
	if err := WriteJSONSuccess(&buf, sps); err != nil {
		t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
	}

	var env struct {
		Status string           `json:"status"`
		Data   []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if env.Status != "success" {
		t.Errorf("expected status 'success', got %q", env.Status)
	}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(env.Data))
	}

	mapped := env.Data[0]
	for _, key := range []string{"entity_id", "acs_url", "acs_binding"} {
		if _, ok := mapped[key]; !ok {
			t.Errorf("expected %q in first entry, got %v", key, mapped)
		}
	}
	if _, ok := mapped["attribute_mapping"]; !ok {
		t.Errorf("expected attribute_mapping in first entry, got %v", mapped)
	}

	unmapped := env.Data[1]
	if _, ok := unmapped["attribute_mapping"]; ok {
		t.Errorf("expected attribute_mapping to be omitted in second entry, got %v", unmapped)
	}
}

func TestSPListJSONEnvelopeEmpty(t *testing.T) {
	sps := make([]*SPListItem, 0)

	var buf bytes.Buffer
	if err := WriteJSONSuccess(&buf, sps); err != nil {
		t.Fatalf("WriteJSONSuccess() unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), `"data":[]`) {
		t.Errorf("expected empty data array (not null), got %q", buf.String())
	}
}

func TestSPListJSONErrorEnvelope(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSONError(&buf, errors.New("cannot list service providers: connection refused")); err != nil {
		t.Fatalf("WriteJSONError() unexpected error: %v", err)
	}

	var env ErrorEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if env.Status != "error" {
		t.Errorf("expected status 'error', got %q", env.Status)
	}
	if !strings.Contains(env.Error, "connection refused") {
		t.Errorf("expected error message to mention failure, got %q", env.Error)
	}
}
