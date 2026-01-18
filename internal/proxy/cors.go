package proxy

import (
	"net/http"
	"strings"
)

// localOrigins are origins that are always allowed for local development
var localOrigins = []string{
	"http://localhost",
	"http://127.0.0.1",
	"http://[::1]",
}

// CORSMiddleware creates a middleware that handles CORS requests.
// It allows local origins by default and any additional origins specified in config.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedOrigin checks if the origin is in the allowed list.
// Local origins (localhost, 127.0.0.1, [::1]) are always allowed.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	// Check local origins first (always allowed)
	// Match exactly or with a port suffix to prevent bypass attacks
	// (e.g., http://localhost.evil.com would bypass prefix matching)
	for _, local := range localOrigins {
		if origin == local || strings.HasPrefix(origin, local+":") {
			return true
		}
	}

	// Check configured origins
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		// Exact match or with port suffix
		if origin == allowed || strings.HasPrefix(origin, allowed+":") {
			return true
		}
	}

	return false
}
