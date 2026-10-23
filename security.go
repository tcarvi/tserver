package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

const (
	defaultRequestsPerSecond = 10
	defaultRateBurst         = 20
	defaultMaxBodyBytes      = 1 << 20
)

type apiConfig struct {
	jwtSecret         []byte
	jwtIssuer         string
	allowedOrigins    map[string]struct{}
	requestsPerSecond rate.Limit
	rateBurst         int
	maxBodyBytes      int64
}

type jwtClaims struct {
	jwt.RegisteredClaims
	Permissions []string `json:"permissions"`
	Scope       string   `json:"scope"`
}

type principal struct {
	subject     string
	permissions map[string]struct{}
}

type principalContextKey struct{}

func apiConfigFromEnv() (apiConfig, error) {
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		return apiConfig{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}

	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		return apiConfig{}, errors.New("JWT_ISSUER is required")
	}

	origins, err := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if err != nil {
		return apiConfig{}, err
	}

	requestsPerSecond, err := positiveFloatEnv("RATE_LIMIT_RPS", defaultRequestsPerSecond)
	if err != nil {
		return apiConfig{}, err
	}
	rateBurst, err := positiveIntEnv("RATE_LIMIT_BURST", defaultRateBurst)
	if err != nil {
		return apiConfig{}, err
	}
	maxBodyBytes, err := positiveInt64Env("MAX_REQUEST_BODY_BYTES", defaultMaxBodyBytes)
	if err != nil {
		return apiConfig{}, err
	}

	return apiConfig{
		jwtSecret:         secret,
		jwtIssuer:         issuer,
		allowedOrigins:    origins,
		requestsPerSecond: rate.Limit(requestsPerSecond),
		rateBurst:         rateBurst,
		maxBodyBytes:      maxBodyBytes,
	}, nil
}

func positiveFloatEnv(name string, defaultValue float64) (float64, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive number", name)
	}
	return parsed, nil
}

func positiveIntEnv(name string, defaultValue int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func positiveInt64Env(name string, defaultValue int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parseAllowedOrigins(value string) (map[string]struct{}, error) {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid CORS origin %q", origin)
		}
		origins[origin] = struct{}{}
	}
	return origins, nil
}

func jwtMiddleware(secret []byte, issuer string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tserver"`)
			writeAPIError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}

		claims := new(jwtClaims)
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
		if err != nil || token == nil || !token.Valid || claims.Subject == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tserver", error="invalid_token"`)
			writeAPIError(w, http.StatusUnauthorized, "invalid or expired bearer token")
			return
		}

		permissions := make(map[string]struct{}, len(claims.Permissions))
		for _, permission := range claims.Permissions {
			permissions[permission] = struct{}{}
		}
		for _, permission := range strings.Fields(claims.Scope) {
			permissions[permission] = struct{}{}
		}
		requestPrincipal := principal{subject: claims.Subject, permissions: permissions}
		ctx := context.WithValue(r.Context(), principalContextKey{}, requestPrincipal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func permissionMiddleware(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPrincipal, ok := r.Context().Value(principalContextKey{}).(principal)
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if _, ok := requestPrincipal.permissions[permission]; !ok {
			writeAPIError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bodyLimitMiddleware(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > maxBytes {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
		_ = r.Body.Close()
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			writeAPIError(w, http.StatusBadRequest, "could not read request body")
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigins map[string]struct{}, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		requestedMethod := r.Header.Get("Access-Control-Request-Method")
		isPreflight := r.Method == http.MethodOptions && requestedMethod != ""

		if origin != "" {
			if _, allowed := allowedOrigins[origin]; !allowed {
				writeAPIError(w, http.StatusForbidden, "origin not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}

		if !isPreflight {
			next.ServeHTTP(w, r)
			return
		}
		if origin == "" || !allowedCORSMethod(requestedMethod) || !allowedCORSHeaders(r.Header.Get("Access-Control-Request-Headers")) {
			writeAPIError(w, http.StatusForbidden, "CORS preflight not allowed")
			return
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
	})
}

func allowedCORSMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

func allowedCORSHeaders(value string) bool {
	for _, header := range strings.Split(value, ",") {
		header = strings.TrimSpace(strings.ToLower(header))
		if header != "" && header != "authorization" && header != "content-type" {
			return false
		}
	}
	return true
}

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu                sync.Mutex
	clients           map[string]*rateLimitEntry
	requestsPerSecond rate.Limit
	burst             int
	lastCleanup       time.Time
}

func newIPRateLimiter(requestsPerSecond rate.Limit, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		clients:           make(map[string]*rateLimitEntry),
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
	}
}

func (l *ipRateLimiter) allow(clientIP string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= time.Minute {
		for ip, entry := range l.clients {
			if now.Sub(entry.lastSeen) >= 10*time.Minute {
				delete(l.clients, ip)
			}
		}
		l.lastCleanup = now
	}

	entry, ok := l.clients[clientIP]
	if !ok {
		entry = &rateLimitEntry{limiter: rate.NewLimiter(l.requestsPerSecond, l.burst)}
		l.clients[clientIP] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func rateLimitMiddleware(limiter *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r.RemoteAddr)) {
			w.Header().Set("Retry-After", "1")
			writeAPIError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: message})
}
