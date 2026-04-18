package auth

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// Crockford-style base32 alphabet, minus visually ambiguous chars.
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// GenerateCode returns a human-readable random code (e.g. "K7M3-Q2XR-9HFN").
// groups * groupLen characters drawn from codeAlphabet.
func GenerateCode(groups, groupLen int) (string, error) {
	buf := make([]byte, groups*groupLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range buf {
		if i > 0 && i%groupLen == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(codeAlphabet[int(v)%len(codeAlphabet)])
	}
	return b.String(), nil
}

// NormalizeCode strips separators/whitespace and uppercases so input is
// resilient to user typing.
func NormalizeCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// GenerateDeviceToken returns a high-entropy random token suitable for
// long-lived device auth. Base32 (no padding) for easy storage.
func GenerateDeviceToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}
