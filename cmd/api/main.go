package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tcarvi/tserver/internal/config"
	"github.com/tcarvi/tserver/internal/handler"
	"github.com/tcarvi/tserver/internal/middleware"
	"github.com/tcarvi/tserver/internal/repository/python"
	"github.com/tcarvi/tserver/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	settings, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	resultsRepository, err := python.NewClient(settings.PythonAPIURL)
	if err != nil {
		return err
	}
	resultsService := service.NewTestResults(resultsRepository)
	resultsHandler := handler.NewTestResults(resultsService)
	resultsHandler = middleware.LimitRequestBody(settings.MaxRequestBody, resultsHandler)
	resultsHandler = middleware.RequirePermission("api:read", resultsHandler)
	resultsHandler = middleware.Authenticate(settings.JWTSecret, settings.JWTIssuer, resultsHandler)
	resultsHandler = middleware.CORS(settings.AllowedOrigins, resultsHandler)
	resultsHandler = middleware.NewIPRateLimiter(settings.RequestsPerSecond, settings.RateBurst).Middleware(resultsHandler)

	server := &http.Server{
		Addr:              settings.Address,
		Handler:           middleware.RequestLogging(middleware.SecurityHeaders(handler.NewRouter(resultsHandler))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("tserver API listening on %s", settings.Address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
