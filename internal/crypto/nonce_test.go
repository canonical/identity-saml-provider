// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package crypto

import (
	"testing"
)

func TestGenerateNonce(t *testing.T) {
	t.Parallel()

	t.Run("returns 32 hex characters", func(t *testing.T) {
		t.Parallel()
		nonce, err := GenerateNonce()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(nonce) != 32 {
			t.Errorf("len(nonce) = %d, want 32", len(nonce))
		}
	})

	t.Run("produces distinct values", func(t *testing.T) {
		t.Parallel()
		a, err := GenerateNonce()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		b, err := GenerateNonce()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a == b {
			t.Error("two calls produced identical nonces")
		}
	})
}

func TestNonceEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "matching values", a: "abc123", b: "abc123", want: true},
		{name: "mismatched values", a: "abc123", b: "xyz789", want: false},
		{name: "different lengths", a: "short", b: "longervalue", want: false},
		{name: "empty strings", a: "", b: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NonceEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("NonceEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestEncodeCookieValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stateValue string
		nonceValue string
		want       string
	}{
		{name: "typical values", stateValue: "state123", nonceValue: "nonce456", want: "state123:nonce456"},
		{name: "hex-encoded values", stateValue: "aabbccdd", nonceValue: "11223344", want: "aabbccdd:11223344"},
		{name: "empty values", stateValue: "", nonceValue: "", want: ":"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EncodeCookieValue(tt.stateValue, tt.nonceValue)
			if got != tt.want {
				t.Errorf("EncodeCookieValue(%q, %q) = %q, want %q", tt.stateValue, tt.nonceValue, got, tt.want)
			}
		})
	}
}

func TestDecodeCookieValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cookie    string
		wantState string
		wantNonce string
		wantErr   bool
	}{
		{
			name:      "valid cookie",
			cookie:    "state123:nonce456",
			wantState: "state123",
			wantNonce: "nonce456",
		},
		{
			name:      "round-trip with EncodeCookieValue",
			cookie:    EncodeCookieValue("aabbcc", "ddeeff"),
			wantState: "aabbcc",
			wantNonce: "ddeeff",
		},
		{name: "missing delimiter", cookie: "nocolon", wantErr: true},
		{name: "empty state", cookie: ":nonce456", wantErr: true},
		{name: "empty nonce", cookie: "state123:", wantErr: true},
		{name: "empty string", cookie: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			state, nonce, err := DecodeCookieValue(tt.cookie)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state != tt.wantState {
				t.Errorf("state = %q, want %q", state, tt.wantState)
			}
			if nonce != tt.wantNonce {
				t.Errorf("nonce = %q, want %q", nonce, tt.wantNonce)
			}
		})
	}
}
