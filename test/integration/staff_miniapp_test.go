package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type staffProjectResponse struct {
	ID             string `json:"id"`
	State          string `json:"state"`
	CollectionPlan []struct {
		ID string `json:"id"`
	} `json:"collection_plan"`
}

type visitResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type analysisResponse struct {
	Summary string `json:"summary"`
}

func TestStaffMiniappVerticalSlice(t *testing.T) {
	baseURL := integrationAPIURL(t)
	assertStatus(t, http.MethodGet, baseURL+"/v1/staff/dashboard", "", http.StatusUnauthorized)

	collector := newTestAPI(t, baseURL)
	project := collector.createStaffProject(t)
	collector.confirmConsent(t, project.ID)

	otherCollector := newIsolatedStaffAPI(t, baseURL)
	assertStatus(t, http.MethodGet, baseURL+"/v1/staff/projects/"+project.ID, otherCollector.token, http.StatusNotFound)

	first := collector.createVisit(t, project, "老人家中")
	collector.uploadVisitAsset(t, project.ID, first.ID, "audio", "first.wav", "audio/wav", []byte("RIFF first interview"))
	collector.uploadVisitAsset(t, project.ID, first.ID, "audio", "family.wav", "audio/wav", []byte("RIFF family interview"))
	collector.uploadVisitAsset(t, project.ID, first.ID, "photo", "first.jpg", "image/jpeg", []byte("\xff\xd8\xff first photo \xff\xd9"))
	collector.uploadVisitAsset(t, project.ID, first.ID, "photo", "school.jpg", "image/jpeg", []byte("\xff\xd8\xff school photo \xff\xd9"))
	collector.uploadVisitAsset(t, project.ID, first.ID, "photo", "factory.jpg", "image/jpeg", []byte("\xff\xd8\xff factory photo \xff\xd9"))
	collector.submitAndWaitForVisitAnalysis(t, first.ID, 60*time.Second)

	second := collector.createVisit(t, project, "社区活动室")
	collector.uploadVisitAsset(t, project.ID, second.ID, "audio", "second.wav", "audio/wav", []byte("RIFF second interview"))
	collector.uploadVisitAsset(t, project.ID, second.ID, "photo", "second.jpg", "image/jpeg", []byte("\xff\xd8\xff second photo \xff\xd9"))
	collector.submitAndWaitForVisitAnalysis(t, second.ID, 60*time.Second)

	firstRun := collector.finalizeProject(t, project.ID)
	duplicateRun := collector.finalizeProject(t, project.ID)
	if duplicateRun.ID != firstRun.ID {
		t.Fatalf("duplicate finalization run = %s, want %s", duplicateRun.ID, firstRun.ID)
	}
	collector.waitForWorkflow(t, project.ID, firstRun.ID, 60*time.Second)
	pdf, _ := collector.downloadLatestArtifact(t, project.ID)
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("artifact is not a PDF")
	}
	assertStatus(t, http.MethodGet, baseURL+"/v1/staff/projects/"+project.ID+"/workflow", otherCollector.token, http.StatusNotFound)
	assertStatus(t, http.MethodGet, baseURL+"/v1/staff/projects/"+project.ID+"/artifacts/latest", otherCollector.token, http.StatusNotFound)
}

func integrationAPIURL(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("TEST_API_URL")
	if baseURL == "" {
		t.Skip("TEST_API_URL is not set")
	}
	return baseURL
}

func newIsolatedStaffAPI(t *testing.T, baseURL string) *testAPI {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for staff isolation assertions")
	}
	secret := os.Getenv("TEST_SESSION_SECRET")
	if secret == "" {
		secret = "sangyu-local-session-secret"
	}
	staffID := uuid.New()
	token := "integration-" + uuid.NewString()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(token))
	tokenHash := hex.EncodeToString(mac.Sum(nil))
	now := time.Now().UTC()
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO staff (id, wechat_openid, display_name, state, created_at, updated_at)
		VALUES ($1, $2, 'Isolation Collector', 'active', $3, $3)`, staffID, "integration-"+uuid.NewString(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO staff_sessions (id, staff_id, token_hash, expires_at, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $5)`, uuid.New(), staffID, tokenHash, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	return &testAPI{baseURL: baseURL, client: &http.Client{Timeout: 15 * time.Second}, token: token}
}

func (api *testAPI) createStaffProject(t *testing.T) staffProjectResponse {
	t.Helper()
	var project staffProjectResponse
	api.json(t, http.MethodPost, "/v1/staff/projects", map[string]any{
		"display_name": "双走访验收老人", "birth_year": 1948, "birth_place": "江苏苏州",
		"long_term_residence": "江苏苏州", "primary_occupation": "纺织工人", "target_edition": "standard",
	}, &project)
	if len(project.CollectionPlan) == 0 {
		t.Fatal("project did not return a collection plan")
	}
	return project
}

func (api *testAPI) confirmConsent(t *testing.T, projectID string) {
	t.Helper()
	api.json(t, http.MethodPost, "/v1/staff/projects/"+projectID+"/consents", map[string]string{
		"confirmed_by": "elder",
	}, nil)
}

func (api *testAPI) createVisit(t *testing.T, project staffProjectResponse, location string) visitResponse {
	t.Helper()
	planIDs := make([]string, 0, len(project.CollectionPlan))
	for _, item := range project.CollectionPlan {
		planIDs = append(planIDs, item.ID)
	}
	var visit visitResponse
	api.json(t, http.MethodPost, "/v1/staff/projects/"+project.ID+"/visits", map[string]any{
		"visited_at": time.Now().UTC().Format(time.RFC3339), "location": location,
		"notes": "端到端验收走访", "plan_item_ids": planIDs,
	}, &visit)
	return visit
}

func (api *testAPI) uploadVisitAsset(t *testing.T, projectID, visitID, kind, filename, contentType string, data []byte) {
	t.Helper()
	var ticket uploadTicket
	api.json(t, http.MethodPost, "/v1/staff/projects/"+projectID+"/assets:initiate", map[string]any{
		"visit_id": visitID, "kind": kind, "source": "direct", "filename": filename,
		"display_name": filename, "content_type": contentType, "size_bytes": len(data),
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

func (api *testAPI) submitAndWaitForVisitAnalysis(t *testing.T, visitID string, timeout time.Duration) {
	t.Helper()
	api.json(t, http.MethodPost, "/v1/staff/visits/"+visitID+":submit", map[string]any{}, nil)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var visit visitResponse
		api.json(t, http.MethodGet, "/v1/staff/visits/"+visitID, nil, &visit)
		if visit.State == "failed" {
			t.Fatalf("visit %s analysis failed", visitID)
		}
		if visit.State == "completed" {
			var analysis analysisResponse
			api.json(t, http.MethodGet, "/v1/staff/visits/"+visitID+"/analysis", nil, &analysis)
			if analysis.Summary == "" {
				t.Fatal("visit analysis summary is empty")
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("visit %s analysis did not complete within %s", visitID, timeout)
}

func (api *testAPI) finalizeProject(t *testing.T, projectID string) workflowResponse {
	t.Helper()
	var run workflowResponse
	api.json(t, http.MethodPost, "/v1/staff/projects/"+projectID+":finalize", map[string]bool{
		"confirm_materials_ready": true,
	}, &run)
	return run
}

func assertStatus(t *testing.T, method, url, token string, want int) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("%s %s status = %d, want %d", method, url, response.StatusCode, want)
	}
}
