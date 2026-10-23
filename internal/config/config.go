package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Address           string
	JWTSecret         []byte
	JWTIssuer         string
	PythonAPIURL      string
	AllowedOrigins    map[string]struct{}
	RequestsPerSecond float64
	RateBurst         int
	MaxRequestBody    int64
}

func LoadFromEnv() (Config, error) {
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		return Config{}, errors.New("JWT_ISSUER is required")
	}
	pythonURL := strings.TrimSpace(os.Getenv("PYTHON_API_URL"))
	if err := validateHTTPURL(pythonURL); err != nil {
		return Config{}, fmt.Errorf("PYTHON_API_URL: %w", err)
	}

	origins, err := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if err != nil {
		return Config{}, err
	}
	rate, err := positiveFloatEnv("RATE_LIMIT_RPS", 10)
	if err != nil {
		return Config{}, err
	}
	burst, err := positiveIntEnv("RATE_LIMIT_BURST", 20)
	if err != nil {
		return Config{}, err
	}
	bodyLimit, err := positiveInt64Env("MAX_REQUEST_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}

	address := strings.TrimSpace(os.Getenv("TSERVER_ADDR"))
	if address == "" {
		address = ":8080"
	}
	return Config{
		Address:           address,
		JWTSecret:         secret,
		JWTIssuer:         issuer,
		PythonAPIURL:      pythonURL,
		AllowedOrigins:    origins,
		RequestsPerSecond: rate,
		RateBurst:         burst,
		MaxRequestBody:    bodyLimit,
	}, nil
}

func validateHTTPURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("must be an absolute HTTP(S) URL without user information or fragment")
	}
	return nil
}

func parseAllowedOrigins(value string) (map[string]struct{}, error) {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid CORS origin %q", origin)
		}
		origins[origin] = struct{}{}
	}
	return origins, nil
}

func positiveFloatEnv(name string, defaultValue float64) (float64, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive number", name)
	}
	return parsed, nil
}

func positiveIntEnv(name string, defaultValue int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func positiveInt64Env(name string, defaultValue int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}
