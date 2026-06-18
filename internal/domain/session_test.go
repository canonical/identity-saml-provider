// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package domain_test

import (
	"testing"
	"time"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

func TestSession_IsExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expireTime time.Time
		want       bool
	}{
		{
			name:       "expired session",
			expireTime: time.Now().Add(-1 * time.Hour),
			want:       true,
		},
		{
			name:       "valid session",
			expireTime: time.Now().Add(1 * time.Hour),
			want:       false,
		},
		{
			name:       "boundary - just expired",
			expireTime: time.Now().Add(-1 * time.Millisecond),
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &domain.Session{
				ExpireTime: tt.expireTime,
			}
			if got := s.IsExpired(); got != tt.want {
				t.Errorf("Session.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_DisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		userCommonName string
		userEmail      string
		want           string
	}{
		{
			name:           "returns UserCommonName when set",
			userCommonName: "Jane Doe",
			userEmail:      "jane@example.com",
			want:           "Jane Doe",
		},
		{
			name:           "falls back to email when UserCommonName is empty",
			userCommonName: "",
			userEmail:      "jane@example.com",
			want:           "jane@example.com",
		},
		{
			name:           "returns empty string when both are empty",
			userCommonName: "",
			userEmail:      "",
			want:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &domain.Session{
				UserCommonName: tt.userCommonName,
				UserEmail:      tt.userEmail,
			}
			if got := s.DisplayName(); got != tt.want {
				t.Errorf("Session.DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSession_CanonicalSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		session *domain.Session
		want    string
		wantOK  bool
	}{
		{
			name: "string sub claim returns value",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"sub": "alice-sub-123"},
			},
			want:   "alice-sub-123",
			wantOK: true,
		},
		{
			name: "missing sub claim",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"email": "x@example.com"},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "empty string sub claim",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"sub": ""},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "non-string sub claim (number)",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"sub": 42},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "nil sub claim",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"sub": nil},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "array sub claim",
			session: &domain.Session{
				RawOIDCClaims: map[string]interface{}{"sub": []interface{}{"a", "b"}},
			},
			want:   "",
			wantOK: false,
		},
		{
			name:    "nil RawOIDCClaims",
			session: &domain.Session{},
			want:    "",
			wantOK:  false,
		},
		{
			name:    "nil session",
			session: nil,
			want:    "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := tt.session.CanonicalSubject()
			if got != tt.want {
				t.Errorf("CanonicalSubject() value = %q, want %q", got, tt.want)
			}
			if ok != tt.wantOK {
				t.Errorf("CanonicalSubject() ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}
