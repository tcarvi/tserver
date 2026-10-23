package middleware

import (
	"context"
	"net/http"
	"strings"

	jwtutil "github.com/tcarvi/tserver/internal/pkg/jwt"
)

type principalContextKey struct{}

func Authenticate(secret []byte, issuer string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tserver"`)
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}

		principal, err := jwtutil.ParseBearerToken(parts[1], secret, issuer)
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tserver", error="invalid_token"`)
			writeError(w, http.StatusUnauthorized, "invalid or expired bearer token")
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := r.Context().Value(principalContextKey{}).(jwtutil.Principal)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if _, ok := principal.Permissions[permission]; !ok {
			writeError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}
