package response

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewTestResultsValidatesAndCopiesJSON(t *testing.T) {
	source := []byte(`{"passed":18}`)
	results, err := NewPythonResult(source)
	if err != nil {
		t.Fatal(err)
	}
	source[2] = 'X'
	if !bytes.Equal(results.Bytes(), []byte(`{"passed":18}`)) {
		t.Fatalf("response DTO changed with source buffer: %s", results.Bytes())
	}
}

func TestNewTestResultsRejectsInvalidJSON(t *testing.T) {
	if _, err := NewPythonResult([]byte("{")); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
}

func TestAsyncTestResponsesExposeOnlyPublicFields(t *testing.T) {
	started, err := json.Marshal(TestStarted{
		TestID:    "run-123",
		Status:    TestStatusQueued,
		StatusURL: "/api/test-runs/run-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(started) != `{"test_id":"run-123","status":"queued","status_url":"/api/test-runs/run-123"}` {
		t.Fatalf("unexpected start response: %s", started)
	}

	results, err := json.Marshal(TestResults{
		TestID:      "run-123",
		Application: "inventory",
		Version:     "1.2.3",
		Status:      TestStatusCompleted,
		Results:     json.RawMessage(`{"passed":18,"failed":2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(results) || bytes.Contains(results, []byte("password")) || bytes.Contains(results, []byte("hash")) {
		t.Fatalf("unexpected or sensitive result response: %s", results)
	}
}
