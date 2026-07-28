package skillrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxResponseBytes = 10 << 20

type RemoteError struct {
	StatusCode int
	Body       string
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("skill runner returned HTTP %d: %s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) Run(ctx context.Context, invocation Invocation) (Result, error) {
	body, err := json.Marshal(invocation)
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/invocations", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, err
	}
	if len(responseBody) > maxResponseBytes {
		return Result{}, fmt.Errorf("skill runner response exceeds %d bytes", maxResponseBytes)
	}
	if response.StatusCode/100 != 2 {
		return Result{}, &RemoteError{StatusCode: response.StatusCode, Body: string(responseBody)}
	}
	var result Result
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return Result{}, err
	}
	if result.InvocationID != invocation.InvocationID {
		return Result{}, fmt.Errorf("invocation ID mismatch: got %q, want %q", result.InvocationID, invocation.InvocationID)
	}
	return result, nil
}
