// Copyright 2026 Canonical Ltd
// SPDX-License-Identifier: AGPL-3.0-only

package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// nonceBytes is the number of random bytes used to generate a nonce (128-bit).
const nonceBytes = 16

// GenerateNonce generates a 128-bit cryptographically random nonce
// and returns it as a 32-character hex-encoded string.
func GenerateNonce() (string, error) {
	b := make([]byte, nonceBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NonceEqual performs a constant-time comparison of two nonce strings.
// It returns true only if a and b are identical.
func NonceEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// EncodeCookieValue encodes a state value and nonce value into a single
// cookie string separated by a colon delimiter.
func EncodeCookieValue(stateValue, nonceValue string) string {
	return stateValue + ":" + nonceValue
}

// DecodeCookieValue splits a cookie value into its state and nonce
// components. Returns an error if the format is invalid.
func DecodeCookieValue(cookie string) (stateValue, nonceValue string, err error) {
	parts := strings.SplitN(cookie, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("malformed oauth_nonce cookie")
	}
	return parts[0], parts[1], nil
}
