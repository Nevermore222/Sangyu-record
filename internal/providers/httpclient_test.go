package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientSubmitsCanonicalJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/jobs" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"provider_job_id":"external-1","state":"submitted"}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client, err := NewHTTPClient(HTTPConfig{
		BaseURL: server.URL, Token: "test-token", AllowedHosts: []string{host}, MaxResponseBytes: 1 << 20,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ref, err := client.Submit(context.Background(), SubmitRequest{RequestID: "request-1", ContractVersion: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if ref.ProviderJobID != "external-1" || !json.Valid(ref.Raw) {
		t.Fatalf("ref = %#v", ref)
	}
}

func TestHTTPClientRejectsBaseURLOutsideAllowList(t *testing.T) {
	_, err := NewHTTPClient(HTTPConfig{
		BaseURL: "http://169.254.169.254", AllowedHosts: []string{"provider.internal"},
	}, http.DefaultClient)
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("error = %v, want ErrHostNotAllowed", err)
	}
}

func TestHTTPClientPollsAndCancelsJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/v1/jobs/job%2F1":
			_, _ = w.Write([]byte(`{"request_id":"request-1","provider_job_id":"job/1","state":"processing"}`))
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/v1/jobs/job%2F1:cancel":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := mustHTTPClient(t, server)
	snapshot, err := client.Status(context.Background(), "job/1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != StateProcessing || !json.Valid(snapshot.Raw) {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if err := client.Cancel(context.Background(), "job/1"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPClientLimitsResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"provider_job_id":"` + strings.Repeat("x", 128) + `","state":"submitted"}`))
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	client, err := NewHTTPClient(HTTPConfig{BaseURL: server.URL, AllowedHosts: []string{host}, MaxResponseBytes: 64}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Submit(context.Background(), SubmitRequest{})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("error = %v, want ErrResponseTooLarge", err)
	}
}

func TestHTTPClientReturnsStructuredRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"slow down"}}`))
	}))
	defer server.Close()
	client := mustHTTPClient(t, server)
	_, err := client.Submit(context.Background(), SubmitRequest{})
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.StatusCode != http.StatusTooManyRequests || remote.Code != "rate_limited" {
		t.Fatalf("error = %#v", err)
	}
}

func mustHTTPClient(t *testing.T, server *httptest.Server) *HTTPClient {
	t.Helper()
	host := strings.TrimPrefix(server.URL, "http://")
	client, err := NewHTTPClient(HTTPConfig{BaseURL: server.URL, AllowedHosts: []string{host}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client
}
