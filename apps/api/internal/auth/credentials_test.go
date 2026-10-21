package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{
		{"minimum", "abcdef1!", true}, {"short", "abcde1!", false},
		{"no digit", "abcdefgh!", false}, {"no symbol", "abcdefgh1", false},
		{"space is not symbol", "abcdef1 ", false}, {"unicode codepoints", "žžžžžž1€", true},
		{"unicode digit does not satisfy ASCII digit", "abcdef١!", false},
		{"maximum", strings.Repeat("a", 126) + "1!", true},
		{"too long", strings.Repeat("a", 127) + "1!", false},
		{"invalid UTF8", "abcdef1!\xff", false}, {"preserve spaces", " abcdef1! ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestIdentityPolicy(t *testing.T) {
	for _, tc := range []struct {
		email, username string
		valid           bool
	}{
		{"Test.Name+Tag@example.com", "Keep-Up_1", true},
		{"bad email", "KeepUp", false}, {"Name <a@example.com>", "KeepUp", false},
		{"a@example.com", "aa", false}, {"a@example.com", strings.Repeat("a", 25), false},
		{"a@example.com", "žiga", false}, {"a@example.com", "has space", false},
	} {
		t.Run(tc.email+tc.username, func(t *testing.T) {
			err := ValidateSignup(SignupInput{tc.email, tc.username, "abcdef1!"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
	if got := NormalizeIdentifier(" Test.Name+Tag@Example.COM "); got != "test.name+tag@example.com" {
		t.Fatal(got)
	}
	if !errors.Is(RequireActive(Session{}), ErrPending) {
		t.Fatal("pending account accepted")
	}
	if err := RequireActive(Session{User: User{Verified: true}}); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordHash(t *testing.T) {
	password := " password1€ "
	first, second := hashPassword(password), hashPassword(password)
	if first == second || strings.Contains(first, password) {
		t.Fatal("hash must use unique salt and hide password")
	}
	if !matchesPassword(password, first) || matchesPassword(strings.TrimSpace(password), first) {
		t.Fatal("password comparison changed input")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=999999999,t=3,p=1$AA$AA", strings.Replace(first, "v=19", "v=20", 1), "$argon2id$v=19$m=1,t=0,p=0$AA$AA"} {
		if matchesPassword(password, bad) {
			t.Fatal("malformed hash accepted")
		}
	}
}
