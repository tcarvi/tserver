package response

import (
	"encoding/json"
	"errors"
)

type TestStatus string

const (
	TestStatusQueued    TestStatus = "queued"
	TestStatusRunning   TestStatus = "running"
	TestStatusCompleted TestStatus = "completed"
	TestStatusFailed    TestStatus = "failed"
)

type TestStarted struct {
	TestID    string     `json:"test_id"`
	Status    TestStatus `json:"status"`
	StatusURL string     `json:"status_url"`
}

type TestResults struct {
	TestID      string          `json:"test_id"`
	Application string          `json:"application"`
	Version     string          `json:"version"`
	Status      TestStatus      `json:"status"`
	Results     json.RawMessage `json:"results,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type PythonResult json.RawMessage

func NewPythonResult(body []byte) (PythonResult, error) {
	if !json.Valid(body) {
		return nil, errors.New("Python result must contain valid JSON")
	}
	return PythonResult(append([]byte(nil), body...)), nil
}

func (r PythonResult) Bytes() []byte {
	return []byte(r)
}

type Error struct {
	Error string `json:"error"`
}
