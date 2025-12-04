package server

import (
	"net/http"
	"strings"
)

// corsMiddleware adds CORS headers for requests from trusted proxies
func corsMiddleware(trustedProxies []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from trusted proxies
			origin := r.Header.Get("Origin")
			if origin != "" && isTrustedOrigin(origin, trustedProxies) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isTrustedOrigin checks if the origin is from a trusted proxy
func isTrustedOrigin(origin string, trustedProxies []string) bool {
	// Simple check - in production, this should be more robust
	for _, proxy := range trustedProxies {
		if strings.Contains(origin, proxy) {
			return true
		}
	}
	return false
}
