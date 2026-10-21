package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

// NormalizeIdentifier preserves dots/plus suffixes; only ASCII email is supported.
func NormalizeIdentifier(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

// ValidateSignup applies the same credential policy to every account creation path.
func ValidateSignup(input SignupInput) error {
	email := strings.TrimSpace(input.Email)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || !strings.Contains(email, "@") {
		return ErrInvalidInput
	}
	for _, r := range email {
		if r > 127 {
			return ErrInvalidInput
		}
	}
	if !usernamePattern.MatchString(input.Username) {
		return ErrInvalidInput
	}
	return ValidatePassword(input.Password)
}

// ValidatePassword counts Unicode code points, without trimming or normalization.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidInput
	}
	n := utf8.RuneCountInString(password)
	if n < 8 || n > 128 {
		return ErrInvalidInput
	}
	digit, special := false, false
	for _, r := range password {
		digit = digit || (r >= '0' && r <= '9')
		special = special || unicode.IsPunct(r) || unicode.IsSymbol(r)
	}
	if !digit || !special {
		return ErrInvalidInput
	}
	return nil
}

// hashPassword stores a versioned Argon2id PHC string with a fresh 128-bit salt.
// Work is bounded by the service's semaphore, including unknown-user checks.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	// crypto/rand.Read always fills the buffer or terminates on entropy failure.
	_, _ = rand.Read(salt)
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=1$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func matchesPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	// Bound stored parameters before allocation, including corrupt database rows.
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}
	values := make([]uint64, 3)
	for i, prefix := range []string{"m=", "t=", "p="} {
		if !strings.HasPrefix(params[i], prefix) {
			return false
		}
		n, err := strconv.ParseUint(strings.TrimPrefix(params[i], prefix), 10, 32)
		if err != nil {
			return false
		}
		values[i] = n
	}
	if values[0] < 8192 || values[0] > 262144 || values[1] < 1 || values[1] > 10 || values[2] < 1 || values[2] > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(values[1]), uint32(values[0]), uint8(values[2]), 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func newToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
