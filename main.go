package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"
)

// decodeBase64UTF8 trims surrounding whitespace, decodes standard or
// URL-safe base64, and verifies the result is valid UTF-8.
func decodeBase64UTF8(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		var urlErr error
		data, urlErr = base64.URLEncoding.DecodeString(s)
		if urlErr != nil {
			return "", fmt.Errorf("invalid base64 (input starts with %q): %v", head(s, 100), err)
		}
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("decoded content is not valid UTF-8 (input starts with %q)", head(s, 100))
	}
	return string(data), nil
}

// head returns at most the first n bytes of s, for error messages.
func head(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
