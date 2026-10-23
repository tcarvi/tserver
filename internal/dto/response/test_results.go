package response

import (
	"encoding/json"
	"errors"
)

type TestResults json.RawMessage

func NewTestResults(body []byte) (TestResults, error) {
	if !json.Valid(body) {
		return nil, errors.New("test results must contain valid JSON")
	}
	return TestResults(append([]byte(nil), body...)), nil
}

func (r TestResults) Bytes() []byte {
	return []byte(r)
}

type Error struct {
	Error string `json:"error"`
}
