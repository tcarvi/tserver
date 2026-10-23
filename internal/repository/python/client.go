package python

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	dtorequest "github.com/tcarvi/tserver/internal/dto/request"
	dtoresponse "github.com/tcarvi/tserver/internal/dto/response"
)

const maxResponseBytes int64 = 4 << 20

type Client struct {
	endpoint *url.URL
	http     *http.Client
}

func NewClient(endpoint string) (*Client, error) {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("Python API URL must be an absolute HTTP(S) URL without user information or fragment")
	}
	return &Client{
		endpoint: parsed,
		http: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) FetchTestResults(ctx context.Context, application, version, requestID string) (dtoresponse.PythonResult, int, error) {
	requestBody, err := json.Marshal(dtorequest.StartTest{Application: application, Version: version})
	if err != nil {
		return nil, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(requestBody))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return nil, response.StatusCode, errors.New("Python API returned a server error")
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
		return nil, response.StatusCode, errors.New("Python API returned a non-JSON response")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if int64(len(body)) > maxResponseBytes || !json.Valid(body) {
		return nil, response.StatusCode, errors.New("Python API returned an invalid JSON response")
	}
	results, err := dtoresponse.NewPythonResult(body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	return results, response.StatusCode, nil
}
