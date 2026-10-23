package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

const testRequestBody = `{"application":"inventory","version":"1.2.3"}`

func TestAPIAuthenticationAndPermissions(t *testing.T) {
	python := jsonPythonServer(http.StatusOK, `{"status":"complete"}`)
	defer python.Close()
	config := testAPIConfig()
	config.pythonAPIURL = python.URL
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
			request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(testRequestBody))
			request.Header.Set("Content-Type", "application/json")
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

	for _, route := range []string{"/", "/index.html", "/app.js", "/layout/main.css", "/logo.svg", "/api/other", "/api/test-results-extra"} {
		t.Run(route, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, route, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
			if !json.Valid(response.Body.Bytes()) {
				t.Fatalf("response is not JSON: %s", response.Body.String())
			}
		})
	}

	response := sendTestRequest(handler, http.MethodGet, "/api/test-results", "", signedReadToken(t))
	if response.Code != http.StatusMethodNotAllowed || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("GET contract: status = %d, want JSON %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestCORSPreflightRunsBeforeAuthentication(t *testing.T) {
	config := testAPIConfig()
	config.allowedOrigins = map[string]struct{}{"https://client.example": {}}
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodOptions, "/api/test-results", nil)
	request.Header.Set("Origin", "https://client.example")
	request.Header.Set("Access-Control-Request-Method", "POST")
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

	request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(testRequestBody))
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestRateLimitIsAppliedPerIP(t *testing.T) {
	python := jsonPythonServer(http.StatusOK, `{"status":"complete"}`)
	defer python.Close()
	config := testAPIConfig()
	config.pythonAPIURL = python.URL
	config.requestsPerSecond = rate.Limit(0.001)
	config.rateBurst = 1
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	token := createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, "")

	for requestNumber, wantStatus := range []int{http.StatusOK, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(testRequestBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != wantStatus {
			t.Fatalf("request %d status = %d, want %d", requestNumber+1, response.Code, wantStatus)
		}
	}
}

func TestRequestBodyLimit(t *testing.T) {
	python := jsonPythonServer(http.StatusOK, `{"status":"complete"}`)
	defer python.Close()
	config := testAPIConfig()
	config.pythonAPIURL = python.URL
	config.maxBodyBytes = 4
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	token := createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, "")
	request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader("12345"))
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = -1
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestPostTestResultsForwardsContractAndReturnsJSON(t *testing.T) {
	type capturedRequest struct {
		method      string
		path        string
		contentType string
		requestID   string
		body        []byte
	}
	captured := make(chan capturedRequest, 1)
	python := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- capturedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			requestID:   r.Header.Get("X-Request-ID"),
			body:        body,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"complete","passed":18,"failed":2}`)
	}))
	defer python.Close()
	handler := newTestHandler(t, python.URL+"/internal/results", testAPIConfig())
	response := sendTestRequest(handler, http.MethodPost, "/api/test-results", testRequestBody, signedReadToken(t))

	if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("status = %d, want JSON %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"passed":18`)) {
		t.Fatalf("Python JSON result was not returned: %s", response.Body.String())
	}
	request := <-captured
	if request.method != http.MethodPost || request.path != "/internal/results" || request.contentType != "application/json" {
		t.Fatalf("unexpected Python request: %+v", request)
	}
	if string(request.body) != testRequestBody {
		t.Fatalf("Python request body = %s, want %s", request.body, testRequestBody)
	}
	if request.requestID == "" || request.requestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("request ID mismatch: upstream=%q response=%q", request.requestID, response.Header().Get("X-Request-ID"))
	}
}

func TestTestResultsRejectsInvalidContract(t *testing.T) {
	python := jsonPythonServer(http.StatusOK, `{"status":"complete"}`)
	defer python.Close()
	handler := newTestHandler(t, python.URL, testAPIConfig())
	token := signedReadToken(t)

	for _, body := range []string{
		`{"application":"inventory","version":"1.2.3","extra":true}`,
		`{"application":"inventory"}`,
		`{"application":"","version":"1.2.3"}`,
		`{"application":42,"version":"1.2.3"}`,
		`{"application":"inventory","version":"1.2.3"}{}`,
		`not-json`,
	} {
		response := sendTestRequest(handler, http.MethodPost, "/api/test-results", body, token)
		if response.Code != http.StatusBadRequest || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("body %s: status = %d, want JSON %d; response=%s", body, response.Code, http.StatusBadRequest, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(testRequestBody))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("missing content type: status = %d, want JSON %d", response.Code, http.StatusUnsupportedMediaType)
	}
}

func TestPythonErrorsBecomeSafeJSONResponses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "non-JSON response", status: http.StatusOK, contentType: "text/plain", body: "ok", wantStatus: http.StatusBadGateway},
		{name: "invalid JSON", status: http.StatusOK, contentType: "application/json", body: "{", wantStatus: http.StatusBadGateway},
		{name: "upstream internal error", status: http.StatusInternalServerError, contentType: "application/json", body: `{"error":"private details"}`, wantStatus: http.StatusBadGateway},
		{name: "upstream not found", status: http.StatusNotFound, contentType: "application/json", body: `{"error":"not found"}`, wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			python := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer python.Close()
			handler := newTestHandler(t, python.URL, testAPIConfig())
			response := sendTestRequest(handler, http.MethodPost, "/api/test-results", testRequestBody, signedReadToken(t))

			if response.Code != test.wantStatus || !json.Valid(response.Body.Bytes()) {
				t.Fatalf("status = %d, want JSON %d; body: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "private details") {
				t.Fatal("upstream internal details leaked to client")
			}
		})
	}
}

func TestProcessingLogsContainIDsWithoutCredentials(t *testing.T) {
	python := jsonPythonServer(http.StatusOK, `{"status":"complete"}`)
	defer python.Close()
	handler := newTestHandler(t, python.URL, testAPIConfig())
	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	token := signedReadToken(t)
	response := sendTestRequest(handler, http.MethodPost, "/api/test-results", testRequestBody, token)

	if response.Code != http.StatusOK || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("request failed or has no request ID: status=%d", response.Code)
	}
	for _, expected := range []string{`"application":"inventory"`, `"version":"1.2.3"`, `"status":200`, `"python_status":200`} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("log is missing %s: %s", expected, logs.String())
		}
	}
	if strings.Contains(logs.String(), token) || strings.Contains(logs.String(), "Bearer ") {
		t.Fatal("access token was written to processing log")
	}
}

func TestAPIConfigRequiresPythonURL(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("JWT_ISSUER", "tserver-test")
	t.Setenv("PYTHON_API_URL", "")
	if _, err := apiConfigFromEnv(); err == nil {
		t.Fatal("apiConfigFromEnv() succeeded without PYTHON_API_URL")
	}
}

func newTestHandler(t *testing.T, pythonURL string, config apiConfig) http.Handler {
	t.Helper()
	config.pythonAPIURL = pythonURL
	handler, err := newAPIHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func jsonPythonServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func sendTestRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func testAPIConfig() apiConfig {
	return apiConfig{
		jwtSecret:         []byte(strings.Repeat("s", 32)),
		jwtIssuer:         "tserver-test",
		pythonAPIURL:      "http://127.0.0.1:8000/test-results",
		allowedOrigins:    map[string]struct{}{},
		requestsPerSecond: rate.Limit(100),
		rateBurst:         20,
		maxBodyBytes:      1024,
	}
}

func signedReadToken(t *testing.T) string {
	t.Helper()
	config := testAPIConfig()
	return createTestToken(t, config.jwtSecret, config.jwtIssuer, time.Now().Add(time.Hour), []string{"api:read"}, "")
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
