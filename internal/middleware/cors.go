package middleware

import (
	"net/http"
	"strings"
)

func CORS(allowedOrigins map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/test-results" {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		requestedMethod := r.Header.Get("Access-Control-Request-Method")
		preflight := r.Method == http.MethodOptions && requestedMethod != ""

		if origin != "" {
			if _, allowed := allowedOrigins[origin]; !allowed {
				writeError(w, http.StatusForbidden, "origin not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if !preflight {
			next.ServeHTTP(w, r)
			return
		}
		if origin == "" || requestedMethod != http.MethodPost || !allowedHeaders(r.Header.Get("Access-Control-Request-Headers")) {
			writeError(w, http.StatusForbidden, "CORS preflight not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", http.MethodPost)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
	})
}

func allowedHeaders(value string) bool {
	for _, header := range strings.Split(value, ",") {
		header = strings.TrimSpace(strings.ToLower(header))
		if header != "" && header != "authorization" && header != "content-type" {
			return false
		}
	}
	return true
}
