package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/tcarvi/tserver/internal/pkg/requestid"
)

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

func RequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create request ID")
			return
		}
		requestID := hex.EncodeToString(id[:])
		ctx := requestid.WithContext(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: w}
		startedAt := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		details := requestid.FromContext(ctx)
		entry, err := json.Marshal(struct {
			RequestID    string `json:"request_id"`
			Method       string `json:"method"`
			Path         string `json:"path"`
			Status       int    `json:"status"`
			DurationMS   int64  `json:"duration_ms"`
			Application  string `json:"application,omitempty"`
			Version      string `json:"version,omitempty"`
			PythonStatus int    `json:"python_status,omitempty"`
		}{details.ID, r.Method, r.URL.Path, status, time.Since(startedAt).Milliseconds(), details.Application, details.Version, details.PythonStatus})
		if err == nil {
			log.Print(string(entry))
		}
	})
}
