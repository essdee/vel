package server

import (
	"net"
	"net/http"
	"net/url"
	"time"

	vel "vel/pkg/vel"
)

// proxyClient is a dedicated HTTP client for proxy routes with a 30-second timeout.
var proxyClient = &http.Client{Timeout: 30 * time.Second}

// isPrivateTarget validates that a proxy target URL points to a private/loopback address.
// Returns true if the target is safe to proxy to (localhost or private IP).
func isPrivateTarget(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Only http and https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	host := u.Hostname()

	// Allow "localhost" explicitly
	if host == "localhost" {
		return true
	}

	// Resolve and check IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}

	for _, ip := range ips {
		if !vel.IsPrivateOrReservedIP(ip) {
			return false
		}
	}
	return len(ips) > 0
}

// stripSensitiveHeaders removes sensitive headers from a proxy request and adds
// standard forwarding headers.
func stripSensitiveHeaders(proxyReq *http.Request, originalReq *http.Request) {
	proxyReq.Header.Del("Authorization")
	proxyReq.Header.Del("Cookie")
	proxyReq.Header.Del("Set-Cookie")
	proxyReq.Header.Del("X-Forwarded-For")

	// Add forwarding headers
	clientIP, _, _ := net.SplitHostPort(originalReq.RemoteAddr)
	if clientIP == "" {
		clientIP = originalReq.RemoteAddr
	}
	proxyReq.Header.Set("X-Forwarded-For", clientIP)
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
}
