package server

import (
	"context"
	"net/http"
	"strings"

	"vel/internal/auth"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const identityContextKey contextKey = "vel_identity"

// GetIdentity extracts the authenticated Identity from the request context.
// Returns nil if the request is unauthenticated.
func GetIdentity(r *http.Request) *auth.Identity {
	v := r.Context().Value(identityContextKey)
	if v == nil {
		return nil
	}
	id, ok := v.(*auth.Identity)
	if !ok {
		return nil
	}
	return id
}

// setIdentity attaches an Identity to the request context.
func setIdentity(r *http.Request, id *auth.Identity) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), identityContextKey, id))
}

// SessionMiddleware checks for a vel_session cookie, loads the session from
// the store, and attaches the Identity to the request context.
// If no cookie or session is invalid, it continues without authentication.
func SessionMiddleware(mgr *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := mgr.GetSession(r)
			if err == nil && sess != nil && !sess.IsExpired() {
				r = setIdentity(r, sess.Identity)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware tries each registered provider if the request doesn't
// already have an Identity (from SessionMiddleware).
// On success with a session-creating provider, it creates a session and
// sets the vel_session cookie.
func AuthMiddleware(mgr *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if already authenticated by session middleware
			if GetIdentity(r) != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Try providers via manager
			identity, _, err := mgr.Authenticate(r)
			if err == nil && identity != nil {
				r = setIdentity(r, identity)

				// Create session for non-stateless providers (not api_key)
				if identity.Provider != "api_key" {
					sess, sessErr := mgr.CreateSession(identity)
					if sessErr == nil {
						setSessionCookie(w, mgr, sess.ID)
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth returns 401 if no Identity in context.
// For browser requests, redirects to login page.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetIdentity(r) == nil {
			if isBrowserRequest(r) {
				redirectPath := r.URL.Path
				if r.URL.RawQuery != "" {
					redirectPath += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "/login?redirect="+redirectPath, http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the role is not "admin".
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetIdentity(r)
		if id == nil || id.Role != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			w.Write([]byte(`{"error":"Forbidden"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireScope checks that the Identity's scopes allow the request.
// Admin role bypasses scope checks.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := GetIdentity(r)
			if id == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"Unauthorized"}`))
				return
			}
			// Admin bypasses scope checks
			if id.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			// Check scopes
			if !auth.CheckScope(id.Scopes, r.Method, r.URL.Path) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(403)
				w.Write([]byte(`{"error":"Forbidden: insufficient scope"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// setSessionCookie sets the vel_session cookie on the response.
func setSessionCookie(w http.ResponseWriter, mgr *auth.AuthManager, sessionID string) {
	maxAge := int(mgr.MaxAge().Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     mgr.CookieName(),
		Value:    sessionID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   maxAge,
	})
}

// clearSessionCookie clears the session cookie.
func clearSessionCookie(w http.ResponseWriter, mgr *auth.AuthManager) {
	http.SetCookie(w, &http.Cookie{
		Name:   mgr.CookieName(),
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
}

// isPublicPath returns true for paths that don't require authentication.
func isPublicPath(path string) bool {
	// Exact matches
	switch path {
	case "/", "/login", "/auth/login", "/api/health", "/api/auth",
		"/auth/magic", "/auth/telegram/callback", "/auth/token",
		"/auth/dev", "/auth/logout", "/favicon.ico", "/robots.txt",
		"/api/auth/magic-link/request":
		return true
	}
	// Prefix matches
	if strings.HasPrefix(path, "/public/") ||
		strings.HasPrefix(path, "/core/vendor/") ||
		strings.HasPrefix(path, "/custom/theme/") ||
		strings.HasPrefix(path, "/relay/") {
		return true
	}
	return false
}

// RequireAuthPaths is a middleware that enforces authentication on all paths
// except those explicitly listed as public. For browser requests to protected
// paths, it redirects to /login. For API requests, it returns 401.
func RequireAuthPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if GetIdentity(r) == nil {
			if isBrowserRequest(r) {
				redirectPath := r.URL.Path
				if r.URL.RawQuery != "" {
					redirectPath += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "/login?redirect="+redirectPath, http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isBrowserRequest checks if the request likely comes from a browser (not API).
func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}
	return !strings.Contains(accept, "application/json")
}
