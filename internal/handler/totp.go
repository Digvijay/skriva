// Package handler — TOTP (RFC 6238) implementation using only stdlib.
// Generates and validates 6-digit time-based one-time passwords
// compatible with Google Authenticator, Authy, etc.
package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA1 is required by TOTP/RFC 6238
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	totpDigits = 6
	totpPeriod = 30 // seconds
	totpWindow = 1  // allow ±1 time step for clock skew
)

// usedTOTPCodes tracks recently used TOTP codes to prevent replay attacks.
// Codes are keyed by "secret:code" and expire after 2 * totpWindow * totpPeriod seconds.
var usedTOTPCodes = struct {
	mu    sync.Mutex
	codes map[string]time.Time
}{codes: make(map[string]time.Time)}

// generateTOTPSecret creates a new random 20-byte TOTP secret, base32-encoded.
func generateTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generating TOTP secret: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(secret), "="), nil
}

// computeTOTP generates a TOTP code for the given secret and time.
func computeTOTP(secret string, t time.Time) (string, error) {
	// Decode base32 secret (add padding if needed)
	secret = strings.ToUpper(strings.TrimSpace(secret))
	if m := len(secret) % 8; m != 0 {
		secret += strings.Repeat("=", 8-m)
	}
	key, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("decoding TOTP secret: %w", err)
	}

	// Calculate time counter (RFC 6238)
	counter := uint64(math.Floor(float64(t.Unix()) / float64(totpPeriod)))

	// HOTP(K, C) per RFC 4226
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0F
	code := int64(binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7FFFFFFF)
	code %= int64(math.Pow10(totpDigits))

	return fmt.Sprintf("%06d", code), nil
}

// validateTOTP checks if the given code matches the secret within the allowed window.
// Each code can only be used once to prevent replay attacks.
func validateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}

	// Check if this code was already used (replay prevention)
	codeKey := secret + ":" + code
	usedTOTPCodes.mu.Lock()
	// Garbage-collect expired entries
	cutoff := time.Now().Add(-time.Duration(2*(totpWindow+1)*totpPeriod) * time.Second)
	for k, t := range usedTOTPCodes.codes {
		if t.Before(cutoff) {
			delete(usedTOTPCodes.codes, k)
		}
	}
	if _, used := usedTOTPCodes.codes[codeKey]; used {
		usedTOTPCodes.mu.Unlock()
		return false // Code already used
	}
	usedTOTPCodes.mu.Unlock()

	now := time.Now()
	for i := -totpWindow; i <= totpWindow; i++ {
		t := now.Add(time.Duration(i*totpPeriod) * time.Second)
		expected, err := computeTOTP(secret, t)
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(expected), []byte(code)) {
			// Mark as used
			usedTOTPCodes.mu.Lock()
			usedTOTPCodes.codes[codeKey] = time.Now()
			usedTOTPCodes.mu.Unlock()
			return true
		}
	}
	return false
}

// totpProvisioningURI builds an otpauth:// URI for authenticator app setup.
func totpProvisioningURI(secret, issuer, account string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d&period=%d",
		issuer, account, secret, issuer, totpDigits, totpPeriod)
}
