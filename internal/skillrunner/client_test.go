package skillrunner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRunsVersionedInvocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/invocations" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"invocation_id":"inv-1","status":"succeeded","output":{"title":"岁月留声"},"evidence_refs":["asset-1#0-12"],"warnings":[],"metrics":{"duration_ms":1}}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	result, err := client.Run(context.Background(), Invocation{
		InvocationID:    "inv-1",
		ContractVersion: "1.0",
		Skill:           SkillRef{Name: "mock-memoir", Version: "0.1.0"},
		Input:           json.RawMessage(`{"project_id":"project-1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSucceeded || len(result.EvidenceRefs) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestClientRejectsMismatchedInvocationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"invocation_id":"other","status":"succeeded","output":{},"evidence_refs":[],"warnings":[],"metrics":{}}`)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, server.Client()).Run(context.Background(), Invocation{InvocationID: "expected"})
	if err == nil {
		t.Fatal("Run() error = nil, want invocation ID mismatch")
	}
}
