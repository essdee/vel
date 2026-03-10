package auth

import "strings"

// CheckScope evaluates whether any of the given scopes allow the specified
// HTTP method + path combination.
//
// Scope formats:
//   - "*"             → matches everything
//   - "GET /path/*"   → matches method + path prefix
//   - "/path/*"       → matches any method + path prefix
//   - "GET /path"     → matches method + exact path
//   - "/path"         → matches any method + exact path
func CheckScope(scopes []string, method, path string) bool {
	for _, scope := range scopes {
		if scope == "*" {
			return true
		}

		scopeMethod := ""
		scopePath := scope

		// Split "METHOD /path" format
		if idx := strings.Index(scope, " "); idx > 0 {
			candidate := scope[:idx]
			// Only treat as method if it's uppercase letters (HTTP method)
			if isHTTPMethod(candidate) {
				scopeMethod = candidate
				scopePath = scope[idx+1:]
			}
		}

		// Check method if specified
		if scopeMethod != "" && scopeMethod != method {
			continue
		}

		// Wildcard path match
		if strings.HasSuffix(scopePath, "/*") {
			prefix := strings.TrimSuffix(scopePath, "/*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}

		// Exact path match
		if path == scopePath {
			return true
		}
	}
	return false
}

func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}
