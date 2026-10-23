// Package main serves the tserver API.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func newAPIHandler(config apiConfig) (http.Handler, error) {
	if len(config.jwtSecret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	if config.jwtIssuer == "" {
		return nil, errors.New("JWT_ISSUER is required")
	}
	if config.requestsPerSecond <= 0 || config.rateBurst < 1 || config.maxBodyBytes < 1 {
		return nil, errors.New("rate limits and request body limit must be positive")
	}
	if _, err := parsePythonAPIURL(config.pythonAPIURL); err != nil {
		return nil, err
	}

	api := newTestResultsHandler(config.pythonAPIURL)
	api = bodyLimitMiddleware(config.maxBodyBytes, api)
	api = permissionMiddleware("api:read", api)
	api = jwtMiddleware(config.jwtSecret, config.jwtIssuer, api)
	api = corsMiddleware(config.allowedOrigins, api)
	api = rateLimitMiddleware(newIPRateLimiter(config.requestsPerSecond, config.rateBurst), api)

	mux := http.NewServeMux()
	mux.Handle("/api/test-results", api)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, "route not found")
	})

	return requestLoggingMiddleware(mux), nil
}

func main() {
	config, err := apiConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	handler, err := newAPIHandler(config)
	if err != nil {
		log.Fatal(err)
	}

	addr := os.Getenv("TSERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("tserver API listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
