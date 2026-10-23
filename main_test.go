package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesAPIOnly(t *testing.T) {
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

	t.Run("api route", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/health.json", nil)
		response := httptest.NewRecorder()

		(THandler{}).ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if !strings.Contains(response.Body.String(), `"status":"ok"`) {
			t.Fatalf("unexpected API body: %s", response.Body.String())
		}
	})

	for _, route := range []string{"/", "/index.html", "/app.js", "/layout/main.css", "/logo.svg"} {
		t.Run(route, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, route, nil)
			response := httptest.NewRecorder()

			(THandler{}).ServeHTTP(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}
