package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthenticateAndAuthorize(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	token := testToken(t, secret, "issuer", []string{"api:read"}, time.Now().Add(time.Hour))
	endpoint := RequirePermission("api:read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	handler := Authenticate(secret, "issuer", endpoint)

	request := httptest.NewRequest(http.MethodPost, "/api/test-results", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid token status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/test-results", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCORSPreflightSkipsAuthentication(t *testing.T) {
	endpoint := Authenticate([]byte(strings.Repeat("s", 32)), "issuer", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	handler := CORS(map[string]struct{}{"https://client.example": {}}, endpoint)
	request := httptest.NewRequest(http.MethodOptions, "/api/test-results", nil)
	request.Header.Set("Origin", "https://client.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestLimitRequestBody(t *testing.T) {
	handler := LimitRequestBody(4, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestIPRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(0.001, 1)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i, want := range []int{http.StatusNoContent, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.RemoteAddr = "192.0.2.10:4567"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("request %d status = %d, want %d", i+1, response.Code, want)
		}
	}
}

func testToken(t *testing.T, secret []byte, issuer string, permissions []string, expiresAt time.Time) string {
	t.Helper()
	claims := struct {
		jwt.RegisteredClaims
		Permissions []string `json:"permissions"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "test-subject",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Permissions: permissions,
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
