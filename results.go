package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	pythonRequestTimeout         = 15 * time.Second
	maxPythonResponseBytes int64 = 4 << 20
)

type testResultsRequest struct {
	Application string `json:"application"`
	Version     string `json:"version"`
}

type processingLogData struct {
	requestID    string
	application  string
	version      string
	pythonStatus int
}

type processingLogContextKey struct{}

func requestIDFromContext(r *http.Request) string {
	if data, ok := r.Context().Value(processingLogContextKey{}).(*processingLogData); ok {
		return data.requestID
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not create request ID")
			return
		}

		requestID := hex.EncodeToString(id[:])
		data := &processingLogData{requestID: requestID}
		ctx := context.WithValue(r.Context(), processingLogContextKey{}, data)
		w.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: w}
		startedAt := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		entry, err := json.Marshal(struct {
			RequestID    string `json:"request_id"`
			Method       string `json:"method"`
			Path         string `json:"path"`
			Status       int    `json:"status"`
			DurationMS   int64  `json:"duration_ms"`
			Application  string `json:"application,omitempty"`
			Version      string `json:"version,omitempty"`
			PythonStatus int    `json:"python_status,omitempty"`
		}{
			RequestID:    data.requestID,
			Method:       r.Method,
			Path:         r.URL.Path,
			Status:       status,
			DurationMS:   time.Since(startedAt).Milliseconds(),
			Application:  data.application,
			Version:      data.version,
			PythonStatus: data.pythonStatus,
		})
		if err == nil {
			log.Print(string(entry))
		}
	})
}

func parsePythonAPIURL(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("PYTHON_API_URL must be an absolute http or https URL without user information or a fragment")
	}
	return parsed, nil
}

func newTestResultsHandler(endpoint string) http.Handler {
	upstreamURL, _ := parsePythonAPIURL(endpoint)
	client := &http.Client{
		Timeout: pythonRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeAPIError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var payload testResultsRequest
		if err := decoder.Decode(&payload); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid request JSON")
			return
		}
		var extra interface{}
		if err := decoder.Decode(&extra); err != io.EOF {
			writeAPIError(w, http.StatusBadRequest, "request must contain exactly one JSON object")
			return
		}

		payload.Application = strings.TrimSpace(payload.Application)
		payload.Version = strings.TrimSpace(payload.Version)
		if payload.Application == "" || payload.Version == "" || len(payload.Application) > 128 || len(payload.Version) > 128 {
			writeAPIError(w, http.StatusBadRequest, "application and version are required and must be at most 128 bytes")
			return
		}
		if data, ok := r.Context().Value(processingLogContextKey{}).(*processingLogData); ok {
			data.application = payload.Application
			data.version = payload.Version
		}

		requestBody, err := json.Marshal(payload)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not encode request")
			return
		}
		upstreamRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL.String(), bytes.NewReader(requestBody))
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not create upstream request")
			return
		}
		upstreamRequest.Header.Set("Content-Type", "application/json")
		upstreamRequest.Header.Set("Accept", "application/json")
		if requestID := requestIDFromContext(r); requestID != "" {
			upstreamRequest.Header.Set("X-Request-ID", requestID)
		}

		upstreamResponse, err := client.Do(upstreamRequest)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, "test results service unavailable")
			return
		}
		defer upstreamResponse.Body.Close()
		if data, ok := r.Context().Value(processingLogContextKey{}).(*processingLogData); ok {
			data.pythonStatus = upstreamResponse.StatusCode
		}
		if upstreamResponse.StatusCode >= http.StatusInternalServerError {
			writeAPIError(w, http.StatusBadGateway, "test results service failed")
			return
		}

		responseMediaType, _, err := mime.ParseMediaType(upstreamResponse.Header.Get("Content-Type"))
		if err != nil || (responseMediaType != "application/json" && !strings.HasSuffix(responseMediaType, "+json")) {
			writeAPIError(w, http.StatusBadGateway, "test results service returned a non-JSON response")
			return
		}
		responseBody, err := io.ReadAll(io.LimitReader(upstreamResponse.Body, maxPythonResponseBytes+1))
		if err != nil || int64(len(responseBody)) > maxPythonResponseBytes || !json.Valid(responseBody) {
			writeAPIError(w, http.StatusBadGateway, "test results service returned an invalid response")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(upstreamResponse.StatusCode)
		if r.Method != http.MethodHead {
			if _, err := w.Write(responseBody); err != nil {
				log.Printf("response write failed request_id=%s error=%q", requestIDFromContext(r), err.Error())
			}
		}
	})
}
