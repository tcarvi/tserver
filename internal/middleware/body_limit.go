package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

func LimitRequestBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > maxBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
		_ = r.Body.Close()
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, "could not read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}
