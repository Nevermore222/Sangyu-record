package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultMaxResponseBytes int64 = 10 << 20

var (
	ErrHostNotAllowed   = errors.New("provider host is not allowed")
	ErrResponseTooLarge = errors.New("provider response is too large")
)

type HTTPConfig struct {
	BaseURL          string
	Token            string
	AllowedHosts     []string
	MaxResponseBytes int64
}

type RemoteError struct {
	StatusCode int
	Code       string
	Body       string
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("provider returned HTTP %d (%s): %s", e.StatusCode, e.Code, e.Body)
}

type HTTPClient struct {
	baseURL          string
	token            string
	http             *http.Client
	maxResponseBytes int64
}

func NewHTTPClient(cfg HTTPConfig, client *http.Client) (*HTTPClient, error) {
	if client == nil {
		return nil, errors.New("HTTP client is required")
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid provider base URL %q", cfg.BaseURL)
	}
	allowedHosts := make(map[string]struct{}, len(cfg.AllowedHosts))
	for _, host := range cfg.AllowedHosts {
		if host = strings.TrimSpace(host); host != "" {
			allowedHosts[strings.ToLower(host)] = struct{}{}
		}
	}
	if _, allowed := allowedHosts[strings.ToLower(parsed.Host)]; !allowed {
		return nil, ErrHostNotAllowed
	}
	limit := cfg.MaxResponseBytes
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	httpClient := *client
	previousRedirectCheck := client.CheckRedirect
	httpClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
			return ErrHostNotAllowed
		}
		if _, allowed := allowedHosts[strings.ToLower(request.URL.Host)]; !allowed {
			return ErrHostNotAllowed
		}
		if previousRedirectCheck != nil {
			return previousRedirectCheck(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), token: cfg.Token, http: &httpClient, maxResponseBytes: limit,
	}, nil
}

func (c *HTTPClient) Submit(ctx context.Context, input SubmitRequest) (JobRef, error) {
	var output JobRef
	raw, err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", input, &output)
	if err != nil {
		return JobRef{}, err
	}
	output.Raw = append(json.RawMessage(nil), raw...)
	return output, nil
}

func (c *HTTPClient) Status(ctx context.Context, providerJobID string) (Snapshot, error) {
	var output Snapshot
	raw, err := c.doJSON(ctx, http.MethodGet, "/v1/jobs/"+url.PathEscape(providerJobID), nil, &output)
	if err != nil {
		return Snapshot{}, err
	}
	output.Raw = append(json.RawMessage(nil), raw...)
	return output, nil
}

func (c *HTTPClient) Cancel(ctx context.Context, providerJobID string) error {
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/jobs/"+url.PathEscape(providerJobID)+":cancel", map[string]any{}, nil)
	return err
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, input, output any) ([]byte, error) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > c.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	if response.StatusCode/100 != 2 {
		return nil, decodeRemoteError(response.StatusCode, raw)
	}
	if output != nil {
		if err := json.Unmarshal(raw, output); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func decodeRemoteError(status int, raw []byte) error {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	body := string(raw)
	if len(body) > 4096 {
		body = body[:4096]
	}
	return &RemoteError{StatusCode: status, Code: envelope.Error.Code, Body: body}
}
