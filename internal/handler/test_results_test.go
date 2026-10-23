package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dtoresponse "github.com/tcarvi/tserver/internal/dto/response"
)

type resultsServiceStub struct {
	response dtoresponse.PythonResult
	status   int
	err      error
	app      string
	version  string
}

func (s *resultsServiceStub) Get(_ context.Context, application, version, _ string) (dtoresponse.PythonResult, int, error) {
	s.app = application
	s.version = version
	return s.response, s.status, s.err
}

func TestTestResultsHandlerValidatesAndReturnsJSON(t *testing.T) {
	service := &resultsServiceStub{response: dtoresponse.PythonResult(`{"tests":{"passed":18}}`), status: http.StatusOK}
	handler := NewTestResults(service)
	request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(`{"application":" inventory ","version":"1.2.3"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("status = %d, response = %s", response.Code, response.Body.String())
	}
	if service.app != "inventory" || service.version != "1.2.3" {
		t.Fatalf("service input = %q/%q", service.app, service.version)
	}
}

func TestRouterHasOnlyTestResultsContract(t *testing.T) {
	service := &resultsServiceStub{response: dtoresponse.PythonResult(`{"ok":true}`), status: http.StatusOK}
	router := NewRouter(NewTestResults(service))
	for _, path := range []string{"/", "/api/other", "/api/test-results/extra"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestTestResultsRejectsInvalidContract(t *testing.T) {
	handler := NewTestResults(&resultsServiceStub{response: dtoresponse.PythonResult(`{}`), status: http.StatusOK})
	for _, body := range []string{`{}`, `{"application":"app","version":""}`, `{"application":"app","version":"1","extra":1}`, `{} {}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/test-results", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}
