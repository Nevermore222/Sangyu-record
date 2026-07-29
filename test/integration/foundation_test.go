package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type testAPI struct {
	baseURL string
	client  *http.Client
}

type projectResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type uploadTicket struct {
	AssetID   string `json:"asset_id"`
	UploadURL string `json:"upload_url"`
}

type workflowResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Nodes []struct {
		Name      string `json:"name"`
		State     string `json:"state"`
		ErrorCode string `json:"error_code"`
	} `json:"nodes"`
}

type artifactResponse struct {
	ObjectKey   string `json:"object_key"`
	DownloadURL string `json:"download_url"`
}

func TestFoundationVerticalSlice(t *testing.T) {
	baseURL := os.Getenv("TEST_API_URL")
	if baseURL == "" {
		t.Skip("TEST_API_URL is not set")
	}

	api := newTestAPI(t, baseURL)
	project := api.createProject(t)
	api.uploadAsset(t, project.ID, "audio", "interview.wav", "audio/wav", []byte("RIFF deterministic interview audio"))
	api.uploadAsset(t, project.ID, "photo", "family-photo.jpg", "image/jpeg", []byte("\xff\xd8\xff deterministic family photo \xff\xd9"))

	run := api.startWorkflow(t, project.ID)
	api.waitForWorkflow(t, project.ID, run.ID, 60*time.Second)
	assertProviderAudit(t, run.ID)
	pdf, artifact := api.downloadLatestArtifact(t, project.ID)
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("artifact is not a PDF")
	}

	t.Logf("project_id=%s artifact_path=%s", project.ID, artifact.ObjectKey)
}

func assertProviderAudit(t *testing.T, runID string) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for Provider audit assertions")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT provider_kind, count(*)
		FROM provider_jobs
		WHERE workflow_run_id = $1
		GROUP BY provider_kind`, runID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		found[kind] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	for _, kind := range []string{"media", "knowledge", "agent"} {
		if found[kind] == 0 {
			t.Fatalf("provider kind %s was not used: %#v", kind, found)
		}
	}

	var providerJobs, archivedJobs int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (
			WHERE state = 'succeeded' AND normalized_output IS NOT NULL
			  AND raw_response_object_key IS NOT NULL AND raw_response_object_key <> ''
		)
		FROM provider_jobs WHERE workflow_run_id = $1`, runID).Scan(&providerJobs, &archivedJobs); err != nil {
		t.Fatal(err)
	}
	if providerJobs != 6 || archivedJobs != providerJobs {
		t.Fatalf("provider jobs = %d, archived normalized jobs = %d", providerJobs, archivedJobs)
	}

	var nodeCount, singleAttemptNodes int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE attempts = 1)
		FROM workflow_nodes WHERE run_id = $1`, runID).Scan(&nodeCount, &singleAttemptNodes); err != nil {
		t.Fatal(err)
	}
	if nodeCount != 7 || singleAttemptNodes != nodeCount {
		t.Fatalf("workflow nodes = %d, single-attempt nodes = %d", nodeCount, singleAttemptNodes)
	}

	var manuscript string
	if err := pool.QueryRow(ctx, `
		SELECT output::text FROM workflow_nodes
		WHERE run_id = $1 AND node_name = 'write_book' AND state = 'succeeded'`, runID).Scan(&manuscript); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(manuscript, "audio-fixture#12-20") || !strings.Contains(manuscript, "K-1978-001") {
		t.Fatalf("manuscript evidence references are incomplete: %s", manuscript)
	}
}

func newTestAPI(t *testing.T, baseURL string) *testAPI {
	t.Helper()
	return &testAPI{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (api *testAPI) createProject(t *testing.T) projectResponse {
	t.Helper()
	body := map[string]any{
		"display_name":        "验收测试老人",
		"birth_year":          1948,
		"birth_place":         "江苏苏州",
		"long_term_residence": "江苏苏州",
		"primary_occupation":  "纺织工人",
		"target_edition":      "standard",
	}
	var project projectResponse
	api.json(t, http.MethodPost, "/v1/staff/projects", body, &project)
	return project
}

func (api *testAPI) uploadAsset(t *testing.T, projectID, kind, filename, contentType string, data []byte) {
	t.Helper()
	var ticket uploadTicket
	api.json(t, http.MethodPost, "/v1/staff/projects/"+projectID+"/assets:initiate", map[string]any{
		"kind": kind, "filename": filename, "content_type": contentType, "size_bytes": len(data),
	}, &ticket)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, ticket.UploadURL, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	api.do(t, req, nil)

	digest := sha256.Sum256(data)
	api.json(t, http.MethodPost, "/v1/staff/assets/"+ticket.AssetID+":complete", map[string]string{
		"sha256": hex.EncodeToString(digest[:]),
	}, nil)
}

func (api *testAPI) startWorkflow(t *testing.T, projectID string) workflowResponse {
	t.Helper()
	var run workflowResponse
	api.json(t, http.MethodPost, "/v1/staff/projects/"+projectID+"/workflow:start", nil, &run)
	return run
}

func (api *testAPI) waitForWorkflow(t *testing.T, projectID, runID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var run workflowResponse
		api.json(t, http.MethodGet, "/v1/staff/projects/"+projectID+"/workflow", nil, &run)
		if run.ID != runID {
			t.Fatalf("latest workflow ID = %s, want %s", run.ID, runID)
		}
		if run.State == "failed" {
			t.Fatalf("workflow failed: %+v", run.Nodes)
		}
		if run.State == "succeeded" {
			var project projectResponse
			api.json(t, http.MethodGet, "/v1/staff/projects/"+projectID, nil, &project)
			if project.State != "completed" {
				t.Fatalf("project state = %s, want completed", project.State)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("workflow %s did not complete within %s", runID, timeout)
}

func (api *testAPI) downloadLatestArtifact(t *testing.T, projectID string) ([]byte, artifactResponse) {
	t.Helper()
	var artifact artifactResponse
	api.json(t, http.MethodGet, "/v1/staff/projects/"+projectID+"/artifacts/latest", nil, &artifact)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, artifact.DownloadURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return api.do(t, req, nil), artifact
}

func (api *testAPI) json(t *testing.T, method, path string, input, output any) {
	t.Helper()
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, api.baseURL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	api.do(t, req, output)
}

func (api *testAPI) do(t *testing.T, req *http.Request, output any) []byte {
	t.Helper()
	response, err := api.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d: %s", req.Method, req.URL, response.StatusCode, body)
	}
	if output != nil {
		if err := json.Unmarshal(body, output); err != nil {
			t.Fatal(fmt.Errorf("decode %s %s: %w", req.Method, req.URL, err))
		}
	}
	return body
}
