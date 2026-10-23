package config

import "testing"

func TestLoadFromEnvValidatesSensitiveConfiguration(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("JWT_ISSUER", "test-issuer")
	t.Setenv("PYTHON_API_URL", "http://python:8000/results")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("TSERVER_ADDR", "127.0.0.1:9090")
	t.Setenv("RATE_LIMIT_RPS", "5")
	t.Setenv("RATE_LIMIT_BURST", "8")
	t.Setenv("MAX_REQUEST_BODY_BYTES", "2048")

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.Address != "127.0.0.1:9090" || config.RequestsPerSecond != 5 || config.RateBurst != 8 || config.MaxRequestBody != 2048 {
		t.Fatalf("unexpected config values: %+v", config)
	}
	if _, ok := config.AllowedOrigins["https://app.example.com"]; !ok {
		t.Fatal("configured CORS origin is missing")
	}
}

func TestLoadFromEnvRejectsMissingSecretsAndInvalidEndpoint(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("JWT_ISSUER", "test-issuer")
	t.Setenv("PYTHON_API_URL", "http://python:8000/results")
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected short JWT_SECRET to be rejected")
	}

	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("PYTHON_API_URL", "file:///tmp/results")
	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected non-HTTP Python endpoint to be rejected")
	}
}
