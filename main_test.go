package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

func TestAPIAuthenticationAndPermissions(t *testing.T) {
	setupAPIFiles(t)
	config := testAPIConfig()
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	missingExpirationToken := createTestTokenWithoutExpiration(t, config.jwtSecret, config.jwtIssuer)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "invalid signature", token: createTestToken(t, []byte(strings.Repeat("x", 32)), config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, ""), wantStatus: http.StatusUnauthorized},
		{name: "expired token", token: createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(-time.Hour), []string{"api:read"}, ""), wantStatus: http.StatusUnauthorized},
		{name: "missing expiration", token: missingExpirationToken, wantStatus: http.StatusUnauthorized},
		{name: "wrong issuer", token: createTestToken(t, config.jwtSecret, "another-issuer", time.Now().Add(time.Hour), []string{"api:read"}, ""), wantStatus: http.StatusUnauthorized},
		{name: "missing permission", token: createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), nil, ""), wantStatus: http.StatusForbidden},
		{name: "scope permission", token: createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), nil, "api:read"), wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/health.json", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestAPIOnlyRoutes(t *testing.T) {
	config := testAPIConfig()
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}

	for _, route := range []string{"/", "/index.html", "/app.js", "/layout/main.css", "/logo.svg", "/apix/health"} {
		t.Run(route, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, route, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}

func TestCORSPreflightRunsBeforeAuthentication(t *testing.T) {
	config := testAPIConfig()
	config.allowedOrigins = map[string]struct{}{"https://client.example": {}}
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodOptions, "/api/health.json", nil)
	request.Header.Set("Origin", "https://client.example")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://client.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}

	request = httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://client.example")
	request.Header.Set("Access-Control-Request-Method", "GET")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("non-API preflight status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestCORSRejectsUnlistedOrigin(t *testing.T) {
	config := testAPIConfig()
	config.allowedOrigins = map[string]struct{}{"https://client.example": {}}
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/health.json", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestRateLimitIsAppliedPerIP(t *testing.T) {
	setupAPIFiles(t)
	config := testAPIConfig()
	config.requestsPerSecond = rate.Limit(0.001)
	config.rateBurst = 1
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	token := createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, "")

	for requestNumber, wantStatus := range []int{http.StatusOK, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodGet, "/api/health.json", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != wantStatus {
			t.Fatalf("request %d status = %d, want %d", requestNumber+1, response.Code, wantStatus)
		}
	}
}

func TestRequestBodyLimit(t *testing.T) {
	config := testAPIConfig()
	config.maxBodyBytes = 4
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	token := createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/health.json", strings.NewReader("12345"))
	request.ContentLength = -1
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func setupAPIFiles(t *testing.T) {
	t.Helper()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	temporaryDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(temporaryDir, "github.com/tcarvi/api"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temporaryDir, "github.com/tcarvi/api/health.json"), []byte(`{"status":"ok"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func testAPIConfig() apiConfig {
	return apiConfig{
		jwtSecret:         []byte(strings.Repeat("s", 32)),
		jwtIssuer:         "tserver-test",
		allowedOrigins:    map[string]struct{}{},
		requestsPerSecond: rate.Limit(100),
		rateBurst:         20,
		maxBodyBytes:      1024,
	}
}

func createTestToken(t *testing.T, secret []byte, issuer string, expiresAt time.Time, permissions []string, scope string) string {
	t.Helper()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Permissions: permissions,
		Scope:       scope,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func createTestTokenWithoutExpiration(t *testing.T, secret []byte, issuer string) string {
	t.Helper()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  issuer,
			Subject: "test-user",
		},
		Permissions: []string{"api:read"},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
