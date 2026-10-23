package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tcarvi/tserver/internal/pkg/requestid"
)

func TestRequestLoggingAddsIDAndProcessedFields(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		details := requestid.FromContext(r.Context())
		details.Application = "inventory"
		details.Version = "1.2.3"
		details.PythonStatus = http.StatusOK
		w.WriteHeader(http.StatusOK)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/test-results", nil))

	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
	for _, expected := range []string{`"application":"inventory"`, `"version":"1.2.3"`, `"python_status":200`} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("log missing %s: %s", expected, output.String())
		}
	}
}
