package request

import (
	"errors"
	"strings"
)

type StartTest struct {
	Application string `json:"application" validate:"required,min=1,max=128"`
	Version     string `json:"version" validate:"required,min=1,max=128"`
}

func (r *StartTest) NormalizeAndValidate() error {
	r.Application = strings.TrimSpace(r.Application)
	r.Version = strings.TrimSpace(r.Version)
	if r.Application == "" || r.Version == "" {
		return errors.New("application and version are required")
	}
	if len(r.Application) > 128 || len(r.Version) > 128 {
		return errors.New("application and version must be at most 128 bytes")
	}
	return nil
}

type GetTestResults struct {
	TestID string `json:"-" validate:"required,min=1,max=128"`
}

func (r GetTestResults) Validate() error {
	if strings.TrimSpace(r.TestID) == "" || len(r.TestID) > 128 {
		return errors.New("test_id is required and must be at most 128 bytes")
	}
	return nil
}
