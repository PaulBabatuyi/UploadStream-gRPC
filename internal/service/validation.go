package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ValidateContentType checks if uploaded data matches declared content type
func ValidateContentType(reader io.Reader, declaredType string) error {
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read magic bytes: %w", err)
	}

	actualType := http.DetectContentType(buffer[:n])

	// Normalize types (handle aliases)
	if !isContentTypeMatch(actualType, declaredType) {
		return fmt.Errorf("content type mismatch: declared=%s, detected=%s",
			declaredType, actualType)
	}

	return nil
}

func isContentTypeMatch(actual, declared string) bool {
	// Exact match
	if actual == declared {
		return true
	}

	// Handle MIME type prefix (e.g., "image/jpeg" matches "image/*")
	actualPrefix := strings.Split(actual, "/")[0]
	declaredPrefix := strings.Split(declared, "/")[0]
	if actualPrefix == declaredPrefix {
		return true
	}

	// Handle common aliases
	aliases := map[string][]string{
		"text/plain":       {"text/plain; charset=utf-8"},
		"application/json": {"text/plain"}, // JSON often detected as text
	}

	if compatibles, ok := aliases[declared]; ok {
		for _, compat := range compatibles {
			if actual == compat {
				return true
			}
		}
	}

	return false
}
