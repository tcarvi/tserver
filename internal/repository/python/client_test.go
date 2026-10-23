package python

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientForwardsContractAndReturnsPythonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/results" || r.Header.Get("X-Request-ID") != "request-123" {
			t.Errorf("unexpected upstream request: %s %s request-id=%s", r.Method, r.URL.Path, r.Header.Get("X-Request-ID"))
		}
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["application"] != "app" || request["version"] != "1.2.3" {
			t.Errorf("unexpected contract: %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"passed":18}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL + "/results")
	if err != nil {
		t.Fatal(err)
	}
	response, status, err := client.FetchTestResults(context.Background(), "app", "1.2.3", "request-123")
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || !json.Valid(response) {
		t.Fatalf("status=%d response=%s", status, response)
	}
}

func TestClientRejectsInvalidPythonResponses(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantError   bool
	}{
		{name: "non JSON", status: http.StatusOK, contentType: "text/plain", body: "ok", wantError: true},
		{name: "malformed JSON", status: http.StatusOK, contentType: "application/json", body: "{", wantError: true},
		{name: "oversized JSON", status: http.StatusOK, contentType: "application/json", body: `{"result":"` + strings.Repeat("x", int(maxResponseBytes)) + `"}`, wantError: true},
		{name: "upstream server error", status: http.StatusInternalServerError, contentType: "application/json", body: `{}`, wantError: true},
		{name: "JSON client error", status: http.StatusNotFound, contentType: "application/json", body: `{"error":"not found"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, status, err := client.FetchTestResults(context.Background(), "app", "1.0", "")
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %v", err, test.wantError)
			}
			if !test.wantError && status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
		})
	}
}
