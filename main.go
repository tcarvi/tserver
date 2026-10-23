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

	api := http.Handler(http.HandlerFunc(tServeAPI))
	api = bodyLimitMiddleware(config.maxBodyBytes, api)
	api = permissionMiddleware("api:read", api)
	api = jwtMiddleware(config.jwtSecret, config.jwtIssuer, api)
	api = rateLimitMiddleware(newIPRateLimiter(config.requestsPerSecond, config.rateBurst), api)

	mux := http.NewServeMux()
	mux.Handle("/api", api)
	mux.Handle("/api/", api)

	return corsMiddleware(config.allowedOrigins, mux), nil
}

func tServeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Language", "pt-br")
	w.Header().Set("key-Code", "00000000001")
	http.ServeFile(w, r, "github.com/tcarvi"+r.URL.Path)
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
