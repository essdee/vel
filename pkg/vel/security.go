package vel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

// SafeRelPath validates and cleans a relative path, rejecting traversal attempts.
// Returns cleaned path or error if it contains .., null bytes, backslashes, or
// resolves outside the intended base directory.
func SafeRelPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("path contains null byte")
	}
	if strings.ContainsRune(path, '\\') {
		return "", fmt.Errorf("path contains backslash")
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains ..")
	}

	cleaned := filepath.Clean(path)

	if strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("path resolves to absolute")
	}
	if strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("path escapes base directory")
	}

	return cleaned, nil
}

// SafeHTTPError writes a generic error to the client and logs the real error
// server-side. Never exposes internal error details to the client.
func SafeHTTPError(w http.ResponseWriter, r *http.Request, statusCode int, publicMsg string, internalErr error) {
	http.Error(w, publicMsg, statusCode)
	if internalErr != nil {
		log.Printf("[error] %s %s: %v (status %d)", r.Method, r.URL.Path, internalErr, statusCode)
	}
}

// GenerateToken generates a cryptographically random hex token of the specified
// byte length. Returns a hex-encoded string (2*nBytes characters).
func GenerateToken(nBytes int) (string, error) {
	if nBytes <= 0 {
		return "", fmt.Errorf("nBytes must be positive")
	}
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}
