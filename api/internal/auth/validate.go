package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidEmail = errors.New("enter a valid email address")
	ErrWeakPassword = errors.New("passwords must be 8 to 128 characters")
	ErrInvalidName  = errors.New("names are 3 to 16 letters, digits or underscores")
	ErrReservedName = errors.New("that name is reserved")
)

const (
	MinPasswordLen = 8
	MaxPasswordLen = 128
	MinNameLen     = 3
	MaxNameLen     = 16
)

// NormalizeEmail trims and lowercases an address and rejects anything that isn't a
// bare addr-spec (no display names or angle brackets).
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 || strings.ContainsAny(email, " <>\"") {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	at := strings.LastIndexByte(email, '@')
	if at < 1 || !strings.Contains(email[at+1:], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

var reservedNames = map[string]bool{
	"admin": true, "administrator": true, "server": true, "system": true,
	"moderator": true, "mod": true, "root": true, "player": true, "anonymous": true,
}

// ValidateDisplayName checks the shape of an in-game name. Uniqueness is
// case-insensitive and enforced by the database.
func ValidateDisplayName(name string) error {
	if len(name) < MinNameLen || len(name) > MaxNameLen {
		return ErrInvalidName
	}
	for _, c := range name {
		ok := c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !ok {
			return ErrInvalidName
		}
	}
	if reservedNames[strings.ToLower(name)] {
		return ErrReservedName
	}
	return nil
}
