package handler

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/tcarvi/tserver/internal/dto/request"
	dtoresponse "github.com/tcarvi/tserver/internal/dto/response"
	"github.com/tcarvi/tserver/internal/pkg/requestid"
)

type ResultsService interface {
	Get(ctx context.Context, application, version, requestID string) (dtoresponse.TestResults, int, error)
}

type TestResults struct {
	service ResultsService
}

func NewTestResults(service ResultsService) http.Handler {
	return &TestResults{service: service}
}

func (h *TestResults) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request request.TestResults
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request JSON")
		return
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain exactly one JSON object")
		return
	}

	if err := request.NormalizeAndValidate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	details := requestid.FromContext(r.Context())
	requestID := ""
	if details != nil {
		details.Application = request.Application
		details.Version = request.Version
		requestID = details.ID
	}
	results, status, err := h.service.Get(r.Context(), request.Application, request.Version, requestID)
	if details != nil {
		details.PythonStatus = status
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "test results service unavailable")
		return
	}
	if !json.Valid(results.Bytes()) {
		writeError(w, http.StatusBadGateway, "test results service returned invalid JSON")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(results.Bytes())
}

func NotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "route not found")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dtoresponse.Error{Error: message})
}
