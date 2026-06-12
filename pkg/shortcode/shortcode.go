// Package shortcode provides Base62 encoding/decoding used to derive
// short URL codes from PostgreSQL auto-increment IDs.
//
// Base62 uses the alphabet [0-9A-Za-z] (62 characters), producing compact,
// URL-safe, case-sensitive codes. E.g. id=125 -> "21".
package shortcode

import (
	"errors"
	"strings"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = uint64(len(alphabet))

// ErrInvalidCharacter is returned by Decode when the input contains a
// character outside the Base62 alphabet.
var ErrInvalidCharacter = errors.New("shortcode: invalid character in code")

// Encode converts a non-negative integer ID into its Base62 string
// representation. Encode(0) returns "0".
func Encode(id uint64) string {
	if id == 0 {
		return string(alphabet[0])
	}

	// Max uint64 needs at most 11 Base62 digits.
	buf := make([]byte, 0, 11)
	for id > 0 {
		remainder := id % base
		buf = append(buf, alphabet[remainder])
		id /= base
	}

	// Digits were appended least-significant first; reverse for correct order.
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}

	return string(buf)
}

// Decode converts a Base62 string back into its integer ID.
// Returns ErrInvalidCharacter if the input contains characters outside
// the Base62 alphabet (e.g. '+', '/', whitespace).
func Decode(code string) (uint64, error) {
	if code == "" {
		return 0, ErrInvalidCharacter
	}

	var id uint64
	for _, ch := range code {
		idx := strings.IndexRune(alphabet, ch)
		if idx < 0 {
			return 0, ErrInvalidCharacter
		}
		id = id*base + uint64(idx)
	}

	return id, nil
}
