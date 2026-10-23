package request

import (
	"errors"
	"strings"
)

type TestResults struct {
	Application string `json:"application"`
	Version     string `json:"version"`
}

func (r *TestResults) NormalizeAndValidate() error {
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
