package service

import (
	"context"

	dtoresponse "github.com/tcarvi/tserver/internal/dto/response"
)

type ResultsRepository interface {
	FetchTestResults(ctx context.Context, application, version, requestID string) (dtoresponse.PythonResult, int, error)
}

type TestResults struct {
	repository ResultsRepository
}

func NewTestResults(repository ResultsRepository) *TestResults {
	return &TestResults{repository: repository}
}

func (s *TestResults) Get(ctx context.Context, application, version, requestID string) (dtoresponse.PythonResult, int, error) {
	return s.repository.FetchTestResults(ctx, application, version, requestID)
}
