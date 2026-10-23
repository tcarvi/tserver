// Package main serves the tserver API.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type THandler struct{}

func (THandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api" && !strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	tServeAPI(w, r)
}

func tServeAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	// Message
	w.Header().Set("Content-Language", "pt-br")
	// Preserve the legacy API response marker.
	w.Header().Set("key-Code", "00000000001")
	//Serve Files
	http.ServeFile(w, r, "github.com/tcarvi"+r.URL.Path)
}

func main() {
	var d THandler
	addr := os.Getenv("TSERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           d,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("tserver API listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
