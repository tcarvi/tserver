package response

import (
	"bytes"
	"testing"
)

func TestNewTestResultsValidatesAndCopiesJSON(t *testing.T) {
	source := []byte(`{"passed":18}`)
	results, err := NewTestResults(source)
	if err != nil {
		t.Fatal(err)
	}
	source[2] = 'X'
	if !bytes.Equal(results.Bytes(), []byte(`{"passed":18}`)) {
		t.Fatalf("response DTO changed with source buffer: %s", results.Bytes())
	}
}

func TestNewTestResultsRejectsInvalidJSON(t *testing.T) {
	if _, err := NewTestResults([]byte("{")); err == nil {
		t.Fatal("expected invalid JSON to be rejected")
	}
}
