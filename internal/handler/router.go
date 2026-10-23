package handler

import "net/http"

func NewRouter(testResults http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/test-results", testResults)
	mux.HandleFunc("/", NotFound)
	return mux
}
