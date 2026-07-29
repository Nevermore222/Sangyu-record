# External AI Provider Boundary Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the in-repository Skill Runner concept with durable asynchronous Media, Knowledge, and Agent Provider API boundaries while preserving the existing project-to-PDF vertical slice.

**Architecture:** Keep the modular Go monolith, PostgreSQL authority, Redis/Asynq queue, MinIO evidence store, and Chromium renderer. Provider adapters submit short HTTP requests, persist external jobs, and release the worker; separate poll jobs or signed callbacks resume the owning workflow node after normalized results pass local validation.

**Tech Stack:** Go 1.26.x, chi v5.3.1, pgx v5.10.0, Asynq v0.26.0, MinIO Go SDK v7.2.1, goose v3.27.3, PostgreSQL 17, Redis 7, MinIO, Node.js 24 mock service, Docker Compose.

## Global Constraints

- The Go backend is the only authority for project, workflow, and delivery state.
- Media, knowledge, and agent capabilities are replaceable asynchronous HTTP Providers; this repository does not implement production models, agents, prompts, vector retrieval, or a public knowledge corpus.
- A Provider cannot connect to the application database, queue, or object store and cannot select the next workflow node.
- Raw audio and photos are available only to the Media Provider through short-lived read-only URLs; Knowledge and Agent Providers receive structured data only.
- Provider payloads use versioned canonical schemas; raw third-party responses are stored separately from normalized business output.
- Callback signatures use HMAC-SHA256, include a timestamp, and reject replays older than five minutes.
- Provider job submission, callback handling, polling, terminal result consumption, and workflow advancement are idempotent.
- The current mini-program is not expanded in this plan; its existing project, upload, workflow-status, and PDF flow must remain compatible.
- Use TDD for every behavior change. Do not delete the existing Skill Runner until the replacement contract and mock are passing.
- Run PowerShell commands from `D:\Sangyu-record`.

---

## Scope Boundary

Included:

- Canonical Provider request, state, result, and error types.
- Independent Go interfaces for Media, Knowledge, and Agent Providers.
- A secure HTTP adapter for the agreed asynchronous job protocol.
- `provider_jobs` and `provider_attempts` persistence.
- MinIO storage for raw Provider responses.
- Polling and signed callback completion paths.
- Workflow suspension and resumption without holding a worker for the external task duration.
- A deterministic Mock Provider that implements all three Provider kinds.
- Migration of the existing deterministic workflow to the new Provider boundary.
- Documentation and container-backed acceptance coverage.

Excluded:

- Real speech, vision, knowledge, or agent service credentials.
- Production task prompts or model selection logic.
- Building or ingesting a shared-memory corpus.
- Staff login, consent records, resumable multipart upload, and the expanded mini-program information architecture.
- Production Provider failover policy beyond persisted retry attempts.

## File Structure

```text
contracts/providers/v1/
  submit.schema.json                 Canonical asynchronous submission envelope
  snapshot.schema.json               Canonical status/result envelope
  callback.schema.json               Signed callback body schema
internal/providers/
  model.go                           Kinds, task types, states, requests, snapshots, interfaces
  normalize.go                       Task-specific canonical output validation
  normalize_test.go                  Canonical result validation tests
  httpclient.go                      HTTP Submit, Status, and Cancel adapter
  httpclient_test.go                 Protocol, allow-list, size, and error tests
internal/providerjobs/
  model.go                           Persisted job, attempt, and terminal outcome types
  repository.go                      PostgreSQL job and attempt transitions
  repository_test.go                 Container-backed transition tests
  rawstore.go                        MinIO raw response storage
  rawstore_test.go                   Raw/normalized separation test
  service.go                         Submit, refresh, callback apply, and consume lifecycle
  service_test.go                    Async lifecycle and idempotency tests
  callback.go                        HMAC verifier and callback HTTP handler
  callback_test.go                   Signature, replay, and duplicate tests
internal/workflow/
  provider_processor.go              Submit Provider work for workflow nodes
  provider_processor_test.go         Node-to-Provider task mapping tests
  worker.go                          Completed-versus-pending processor outcomes
  worker_test.go                     Suspend/resume/idempotency tests
  tasks.go                           Workflow-node and Provider-poll Asynq tasks
  repository.go                      Finish nodes from consumed Provider outcomes
migrations/
  00002_provider_jobs.sql            Provider job, attempt, and node-link schema
providers/mock/
  Dockerfile                         Read-only non-root mock image
  package.json                       Mock Provider package and tests
  server.mjs                         Submit/status/cancel implementation
  server.test.mjs                    Success and failure-mode tests
test/fixtures/providers/
  submit-transcription.json          Contract fixture
  callback-succeeded.json            Callback fixture
cmd/api/main.go                      Callback route composition
cmd/worker/main.go                   Provider registry and poll worker composition
internal/config/config.go            Three Provider URLs/tokens and callback secret
deploy/local/compose.yaml             Mock Provider service and application settings
test/integration/foundation_test.go   Provider-backed end-to-end assertions
scripts/vertical-slice.ps1            Mock Provider contract and full-stack gate
README.md                             External capability ownership and local setup
```

### Task 1: Define canonical Provider contracts and normalization

**Files:**
- Create: `internal/providers/model.go`
- Create: `internal/providers/normalize.go`
- Create: `internal/providers/normalize_test.go`
- Create: `contracts/providers/v1/submit.schema.json`
- Create: `contracts/providers/v1/snapshot.schema.json`
- Create: `contracts/providers/v1/callback.schema.json`
- Create: `test/fixtures/providers/submit-transcription.json`
- Create: `test/fixtures/providers/callback-succeeded.json`

**Interfaces:**
- Produces: `providers.MediaProvider`, `providers.KnowledgeProvider`, and `providers.AgentProvider`.
- Produces: `providers.Registry.For(providers.Kind) (providers.Provider, error)`.
- Produces: `providers.Normalize(providers.TaskType, json.RawMessage) (json.RawMessage, error)`.
- Consumed later by: HTTP adapters, Provider job service, workflow Provider processors, and callback handler.

- [ ] **Step 1: Write failing contract and normalization tests**

Create `internal/providers/normalize_test.go`:

```go
package providers

import (
    "encoding/json"
    "errors"
    "testing"
)

func TestNormalizeTranscriptionRequiresEvidenceSource(t *testing.T) {
    _, err := Normalize(TaskAudioTranscription, json.RawMessage(`{
        "segments":[{"start_seconds":0,"end_seconds":4,"text":"我小时候住在苏州。","source_ref":""}]
    }`))
    if !errors.Is(err, ErrInvalidOutput) {
        t.Fatalf("error = %v, want ErrInvalidOutput", err)
    }
}

func TestNormalizeSharedMemoryKeepsKnowledgeCitation(t *testing.T) {
    input := json.RawMessage(`{
        "entries":[{"reference_id":"K-1978-SZ-1","text":"背景资料","source":"苏州地方志",` +
        `"year_from":1978,"year_to":1982,"region":"江苏苏州","confidence":"high","license":"internal-use"}]
    }`)
    normalized, err := Normalize(TaskSharedMemoryRetrieval, input)
    if err != nil {
        t.Fatal(err)
    }
    if !json.Valid(normalized) {
        t.Fatal("normalized result is not JSON")
    }
}
```

- [ ] **Step 2: Run the tests and verify the package is missing**

Run:

```powershell
go test ./internal/providers -run TestNormalize -v
```

Expected: FAIL because `internal/providers` and `Normalize` do not exist.

- [ ] **Step 3: Implement the canonical Go types**

Create `internal/providers/model.go` with these exact public types:

```go
package providers

import (
    "context"
    "encoding/json"
    "errors"
    "time"
)

var (
    ErrKindNotConfigured = errors.New("provider kind is not configured")
    ErrInvalidOutput     = errors.New("provider output is invalid")
)

type Kind string

const (
    KindMedia     Kind = "media"
    KindKnowledge Kind = "knowledge"
    KindAgent     Kind = "agent"
)

type TaskType string

const (
    TaskAudioTranscription     TaskType = "audio_transcription"
    TaskSpeakerDiarization     TaskType = "speaker_diarization"
    TaskPhotoOCR               TaskType = "photo_ocr"
    TaskPhotoUnderstanding     TaskType = "photo_understanding"
    TaskCollectionPlan         TaskType = "collection_plan"
    TaskMaterialAssessment     TaskType = "material_assessment"
    TaskFollowupPlan           TaskType = "followup_plan"
    TaskMemoirPositioning      TaskType = "memoir_positioning"
    TaskTimelineBuilder        TaskType = "timeline_builder"
    TaskChapterPlanner         TaskType = "chapter_planner"
    TaskChapterWriter          TaskType = "chapter_writer"
    TaskChapterReview          TaskType = "chapter_review"
    TaskBookConsistencyReview TaskType = "book_consistency_review"
    TaskSharedMemoryRetrieval  TaskType = "shared_memory_retrieval"
)

type State string

const (
    StatePendingSubmission State = "pending_submission"
    StateSubmitted         State = "submitted"
    StateProcessing        State = "processing"
    StateSucceeded         State = "succeeded"
    StateRetryableFailed   State = "retryable_failed"
    StatePermanentlyFailed State = "permanently_failed"
    StateTimedOut          State = "timed_out"
    StateCancelled         State = "cancelled"
)

type SubmitRequest struct {
    RequestID           string          `json:"request_id"`
    IdempotencyKey      string          `json:"idempotency_key"`
    ContractVersion     string          `json:"contract_version"`
    TaskType            TaskType        `json:"task_type"`
    InputSchemaVersion  string          `json:"input_schema_version"`
    OutputSchemaVersion string          `json:"output_schema_version"`
    Input               json.RawMessage `json:"input"`
    ResourceURLs        []string        `json:"resource_urls"`
    CallbackURL         string          `json:"callback_url"`
    Deadline            time.Time       `json:"deadline"`
}

type JobRef struct {
    ProviderJobID string `json:"provider_job_id"`
    State         State  `json:"state"`
    Raw           json.RawMessage `json:"-"`
}

type Snapshot struct {
    RequestID     string          `json:"request_id"`
    ProviderJobID string          `json:"provider_job_id"`
    State         State           `json:"state"`
    Output        json.RawMessage `json:"output,omitempty"`
    ErrorCode     string          `json:"error_code,omitempty"`
    ErrorMessage  string          `json:"error_message,omitempty"`
    RetryAfter    time.Duration   `json:"-"`
    Raw           json.RawMessage `json:"-"`
}

type Provider interface {
    Submit(context.Context, SubmitRequest) (JobRef, error)
    Status(context.Context, string) (Snapshot, error)
    Cancel(context.Context, string) error
}

type MediaProvider interface{ Provider }
type KnowledgeProvider interface{ Provider }
type AgentProvider interface{ Provider }

type Registry struct {
    Media     MediaProvider
    Knowledge KnowledgeProvider
    Agent     AgentProvider
}

func (r Registry) For(kind Kind) (Provider, error) {
    switch kind {
    case KindMedia:
        if r.Media != nil { return r.Media, nil }
    case KindKnowledge:
        if r.Knowledge != nil { return r.Knowledge, nil }
    case KindAgent:
        if r.Agent != nil { return r.Agent, nil }
    }
    return nil, ErrKindNotConfigured
}
```

- [ ] **Step 4: Implement task-specific normalization**

Create `internal/providers/normalize.go`. Decode into concrete structs and reject empty evidence identifiers. The implementation must cover every task used by the foundation workflow:

```go
package providers

import (
    "bytes"
    "encoding/json"
    "fmt"
)

type Transcript struct {
    Segments []TranscriptSegment `json:"segments"`
}

type TranscriptSegment struct {
    StartSeconds float64 `json:"start_seconds"`
    EndSeconds   float64 `json:"end_seconds"`
    Text         string  `json:"text"`
    SourceRef    string  `json:"source_ref"`
}

type KnowledgeResult struct {
    Entries []KnowledgeEntry `json:"entries"`
}

type KnowledgeEntry struct {
    ReferenceID string `json:"reference_id"`
    Text        string `json:"text"`
    Source      string `json:"source"`
    YearFrom    int    `json:"year_from"`
    YearTo      int    `json:"year_to"`
    Region      string `json:"region"`
    Confidence  string `json:"confidence"`
    License     string `json:"license"`
}

func Normalize(task TaskType, raw json.RawMessage) (json.RawMessage, error) {
    if !json.Valid(raw) {
        return nil, fmt.Errorf("%w: malformed JSON", ErrInvalidOutput)
    }
    switch task {
    case TaskAudioTranscription:
        var value Transcript
        if err := json.Unmarshal(raw, &value); err != nil || len(value.Segments) == 0 {
            return nil, fmt.Errorf("%w: transcription has no segments", ErrInvalidOutput)
        }
        for _, segment := range value.Segments {
            if segment.Text == "" || segment.SourceRef == "" || segment.EndSeconds < segment.StartSeconds {
                return nil, fmt.Errorf("%w: invalid transcription segment", ErrInvalidOutput)
            }
        }
    case TaskSharedMemoryRetrieval:
        var value KnowledgeResult
        if err := json.Unmarshal(raw, &value); err != nil || len(value.Entries) == 0 {
            return nil, fmt.Errorf("%w: knowledge result has no entries", ErrInvalidOutput)
        }
        for _, entry := range value.Entries {
            if entry.ReferenceID == "" || entry.Source == "" || entry.License == "" {
                return nil, fmt.Errorf("%w: knowledge citation is incomplete", ErrInvalidOutput)
            }
        }
    case TaskPhotoUnderstanding, TaskTimelineBuilder, TaskChapterPlanner, TaskChapterWriter:
        var value map[string]any
        if err := json.Unmarshal(raw, &value); err != nil || len(value) == 0 {
            return nil, fmt.Errorf("%w: task output is empty", ErrInvalidOutput)
        }
    default:
        return nil, fmt.Errorf("%w: unsupported task %q", ErrInvalidOutput, task)
    }
    buffer := bytes.NewBuffer(make([]byte, 0, len(raw)))
    if err := json.Compact(buffer, raw); err != nil { return nil, err }
    return buffer.Bytes(), nil
}
```

Do not relax unsupported task handling; additional task validators belong to later feature plans.

- [ ] **Step 5: Add the three JSON Schema files and fixtures**

`submit.schema.json` must require the fields in `SubmitRequest`, set `contract_version` to `1.0`, restrict `task_type` to the constants above, and set `additionalProperties` to `false`. `snapshot.schema.json` must require `request_id`, `provider_job_id`, `state`, and nullable `output`; terminal failure states require `error_code`. `callback.schema.json` uses the same body as `snapshot.schema.json`; transport signature data stays in HTTP headers.

Validate the fixtures with Node's JSON parser:

```powershell
node -e "for (const f of process.argv.slice(1)) JSON.parse(require('fs').readFileSync(f,'utf8'))" test/fixtures/providers/submit-transcription.json test/fixtures/providers/callback-succeeded.json
```

Expected: exit code 0 with no output.

- [ ] **Step 6: Run focused and repository tests**

Run:

```powershell
go test ./internal/providers -v
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit canonical contracts**

```powershell
git add internal/providers contracts/providers test/fixtures/providers
git commit -m "feat: define external provider contracts"
```

### Task 2: Implement the secure asynchronous HTTP Provider adapter

**Files:**
- Create: `internal/providers/httpclient.go`
- Create: `internal/providers/httpclient_test.go`

**Interfaces:**
- Consumes: `providers.SubmitRequest`, `providers.JobRef`, and `providers.Snapshot` from Task 1.
- Produces: `providers.NewHTTPClient(providers.HTTPConfig, *http.Client) (*providers.HTTPClient, error)`.
- Produces: one `*providers.HTTPClient` that satisfies each of the three named Provider interfaces.

- [ ] **Step 1: Write failing HTTP protocol tests**

Create tests for exact routes, bearer authentication, response limits, and host validation:

```go
func TestHTTPClientSubmitsCanonicalJob(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost || r.URL.Path != "/v1/jobs" {
            t.Fatalf("request = %s %s", r.Method, r.URL.Path)
        }
        if r.Header.Get("Authorization") != "Bearer test-token" {
            t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
        }
        json.NewEncoder(w).Encode(JobRef{ProviderJobID: "external-1", State: StateSubmitted})
    }))
    defer server.Close()

    host := strings.TrimPrefix(server.URL, "http://")
    client, err := NewHTTPClient(HTTPConfig{
        BaseURL: server.URL, Token: "test-token", AllowedHosts: []string{host}, MaxResponseBytes: 1 << 20,
    }, server.Client())
    if err != nil { t.Fatal(err) }
    ref, err := client.Submit(context.Background(), SubmitRequest{RequestID: "request-1", ContractVersion: "1.0"})
    if err != nil { t.Fatal(err) }
    if ref.ProviderJobID != "external-1" { t.Fatalf("ref = %#v", ref) }
}

func TestHTTPClientRejectsBaseURLOutsideAllowList(t *testing.T) {
    _, err := NewHTTPClient(HTTPConfig{BaseURL: "http://169.254.169.254", AllowedHosts: []string{"provider.internal"}}, http.DefaultClient)
    if !errors.Is(err, ErrHostNotAllowed) { t.Fatalf("error = %v", err) }
}
```

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/providers -run TestHTTPClient -v`

Expected: FAIL because `NewHTTPClient`, `HTTPConfig`, and `ErrHostNotAllowed` do not exist.

- [ ] **Step 3: Implement configuration validation and shared request execution**

Use this public configuration:

```go
type HTTPConfig struct {
    BaseURL         string
    Token           string
    AllowedHosts    []string
    MaxResponseBytes int64
}
```

`NewHTTPClient` must parse the URL, allow only `http` or `https`, compare `URL.Host` case-insensitively against `AllowedHosts`, trim the trailing slash, default the response limit to 10 MiB, and reject a nil `*http.Client`. Define typed errors `ErrHostNotAllowed`, `ErrResponseTooLarge`, and `RemoteError{StatusCode, Code, Body}`.

Implement these routes:

```go
func (c *HTTPClient) Submit(ctx context.Context, input SubmitRequest) (JobRef, error) {
    var output JobRef
    err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", input, &output)
    return output, err
}

func (c *HTTPClient) Status(ctx context.Context, providerJobID string) (Snapshot, error) {
    var output Snapshot
    path := "/v1/jobs/" + url.PathEscape(providerJobID)
    err := c.doJSON(ctx, http.MethodGet, path, nil, &output)
    return output, err
}

func (c *HTTPClient) Cancel(ctx context.Context, providerJobID string) error {
    path := "/v1/jobs/" + url.PathEscape(providerJobID) + ":cancel"
    return c.doJSON(ctx, http.MethodPost, path, map[string]any{}, nil)
}
```

`doJSON` must set `Authorization: Bearer <token>` only when a token exists, always set `Accept: application/json`, set `Content-Type` for a request body, use `io.LimitReader(max+1)`, and include at most 4 KiB of a non-2xx response body in `RemoteError`.

`Submit` copies the exact success response bytes into `JobRef.Raw`; `Status` copies them into `Snapshot.Raw`. The job service archives those bytes without reconstructing third-party JSON from decoded structs.

- [ ] **Step 4: Cover malformed JSON, excessive responses, cancellation, and status polling**

Add table tests that verify:

- `GET /v1/jobs/{id}` decodes a processing snapshot.
- `POST /v1/jobs/{id}:cancel` accepts 204.
- an 11 MiB response returns `ErrResponseTooLarge` when the limit is 10 MiB.
- malformed success JSON is returned as a decode error.
- a 429 response is a `*RemoteError` with code `rate_limited`.

- [ ] **Step 5: Run tests and commit**

```powershell
go test ./internal/providers -v
go test ./...
git add internal/providers/httpclient.go internal/providers/httpclient_test.go
git commit -m "feat: add HTTP provider adapter"
```

Expected: all tests PASS and the commit succeeds.

### Task 3: Persist Provider jobs, attempts, and raw responses

**Files:**
- Create: `migrations/00002_provider_jobs.sql`
- Create: `internal/providerjobs/model.go`
- Create: `internal/providerjobs/repository.go`
- Create: `internal/providerjobs/repository_test.go`
- Create: `internal/providerjobs/rawstore.go`
- Create: `internal/providerjobs/rawstore_test.go`
- Modify: `internal/workflow/model.go`

**Interfaces:**
- Produces: `providerjobs.Repository` and `providerjobs.PostgresRepository`.
- Produces: `providerjobs.RawStore.Put(ctx, job, attempt, raw) (objectKey string, error)`.
- Produces: atomic `ConsumeTerminal(ctx, jobID) (providerjobs.Outcome, bool, error)`.
- Consumes: existing workflow run IDs, node names, MinIO client, and bucket.

- [ ] **Step 1: Write the failing repository transition test**

Add a container-backed test guarded by `TEST_DATABASE_URL`:

```go
func TestTerminalProviderJobIsConsumedOnce(t *testing.T) {
    repo := openTestRepository(t)
    job := insertProviderJobFixture(t, repo)
    if err := repo.ApplySnapshot(context.Background(), job.ID, providers.Snapshot{
        RequestID: job.RequestID.String(), ProviderJobID: "external-1", State: providers.StateSucceeded,
        Output: json.RawMessage(`{"segments":[{"start_seconds":0,"end_seconds":1,"text":"测试","source_ref":"audio#0-1"}]}`),
    }, "raw/provider-response.json"); err != nil { t.Fatal(err) }

    _, consumed, err := repo.ConsumeTerminal(context.Background(), job.ID)
    if err != nil || !consumed { t.Fatalf("first consume = %v, %v", consumed, err) }
    _, consumed, err = repo.ConsumeTerminal(context.Background(), job.ID)
    if err != nil || consumed { t.Fatalf("second consume = %v, %v", consumed, err) }
}
```

- [ ] **Step 2: Run the focused test and verify failure**

Run with the local database URL:

```powershell
$env:TEST_DATABASE_URL='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go test ./internal/providerjobs -run TestTerminalProviderJobIsConsumedOnce -v
```

Expected: FAIL because the package and migration do not exist.

- [ ] **Step 3: Add the migration**

Create `migrations/00002_provider_jobs.sql` with reversible goose sections. The Up section must create:

```sql
CREATE TYPE provider_kind AS ENUM ('media', 'knowledge', 'agent');
CREATE TYPE provider_job_state AS ENUM (
    'pending_submission', 'submitted', 'processing', 'succeeded',
    'retryable_failed', 'permanently_failed', 'timed_out', 'cancelled'
);

CREATE TABLE provider_jobs (
    id uuid PRIMARY KEY,
    request_id uuid NOT NULL UNIQUE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workflow_run_id uuid NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    workflow_node_name text NOT NULL,
    provider_kind provider_kind NOT NULL,
    task_type text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    provider_job_id text,
    state provider_job_state NOT NULL DEFAULT 'pending_submission',
    input jsonb NOT NULL,
    normalized_output jsonb,
    raw_response_object_key text,
    error_code text,
    error_message text,
    consumed_at timestamptz,
    deadline timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_kind, provider_job_id)
);

CREATE TABLE provider_attempts (
    id uuid PRIMARY KEY,
    provider_job_id uuid NOT NULL REFERENCES provider_jobs(id) ON DELETE CASCADE,
    attempt integer NOT NULL CHECK (attempt > 0),
    operation text NOT NULL CHECK (operation IN ('submit', 'status', 'callback', 'cancel')),
    state provider_job_state NOT NULL,
    http_status integer,
    raw_response_object_key text,
    error_code text,
    elapsed_ms bigint NOT NULL CHECK (elapsed_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_job_id, attempt)
);

CREATE INDEX provider_jobs_workflow_node_idx ON provider_jobs(workflow_run_id, workflow_node_name);
CREATE INDEX provider_jobs_poll_idx ON provider_jobs(state, updated_at);
```

The Down section drops attempts, jobs, and both enum types in dependency order.

- [ ] **Step 4: Define persisted types and repository interface**

Create `internal/providerjobs/model.go` with `Job`, `Attempt`, `CreateInput`, and:

```go
type Outcome struct {
    JobID        uuid.UUID
    ProjectID    uuid.UUID
    WorkflowRunID uuid.UUID
    WorkflowNode string
    State        providers.State
    Output       json.RawMessage
    ErrorCode    string
}

type Repository interface {
    CreateOrGet(context.Context, CreateInput) (Job, bool, error)
    MarkSubmitted(context.Context, uuid.UUID, providers.JobRef) error
    Get(context.Context, uuid.UUID) (Job, error)
    AddAttempt(context.Context, Attempt) error
    ApplySnapshot(context.Context, uuid.UUID, providers.Snapshot, string) error
    ConsumeTerminal(context.Context, uuid.UUID) (Outcome, bool, error)
}
```

`CreateOrGet` returns `created=false` for an existing idempotency key. `ApplySnapshot` never replaces a terminal state with another state. `retryable_failed` is non-terminal and may transition back to `processing` or to a terminal state. `ConsumeTerminal` uses one `UPDATE ... WHERE consumed_at IS NULL AND state IN ('succeeded','permanently_failed','timed_out','cancelled') RETURNING ...` transaction.

- [ ] **Step 5: Implement raw response object storage**

Create `internal/providerjobs/rawstore.go`:

```go
type RawStore interface {
    Put(context.Context, Job, int, []byte) (string, error)
}

type MinioRawStore struct {
    client *minio.Client
    bucket string
}

func (s *MinioRawStore) Put(ctx context.Context, job Job, attempt int, raw []byte) (string, error) {
    key := fmt.Sprintf("projects/%s/provider-jobs/%s/attempts/%d.json", job.ProjectID, job.ID, attempt)
    _, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: "application/json"})
    return key, err
}
```

Test that the returned key is project-scoped and that normalized output is not written into the raw object path by the repository.

- [ ] **Step 6: Apply the migration and run tests**

```powershell
$env:GOOSE_DRIVER='postgres'
$env:GOOSE_DBSTRING='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
$env:TEST_DATABASE_URL=$env:GOOSE_DBSTRING
go test ./internal/providerjobs -v
go test ./...
```

Expected: migration 2 is applied and tests PASS.

- [ ] **Step 7: Commit persistence**

```powershell
git add migrations/00002_provider_jobs.sql internal/providerjobs internal/workflow/model.go
git commit -m "feat: persist asynchronous provider jobs"
```

### Task 4: Implement Provider job lifecycle and signed callbacks

**Files:**
- Create: `internal/providerjobs/service.go`
- Create: `internal/providerjobs/service_test.go`
- Create: `internal/providerjobs/callback.go`
- Create: `internal/providerjobs/callback_test.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Consumes: `providers.Registry`, `providerjobs.Repository`, `providerjobs.RawStore`.
- Produces: `providerjobs.Service.Submit`, `Refresh`, `ApplyCallback`, and `ConsumeTerminal`.
- Produces: `providerjobs.CallbackHandler.Register(chi.Router)` at `/v1/provider-callbacks/{kind}/{jobID}`.
- Produces: `providerjobs.PollEnqueuer.EnqueueProviderPoll(context.Context, uuid.UUID, time.Duration) error`.

- [ ] **Step 1: Write failing asynchronous lifecycle tests**

Use in-memory fakes to prove submit idempotency and callback/poll convergence:

```go
func TestSubmitUsesPersistedIdempotencyKey(t *testing.T) {
    provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
    repo := newMemoryRepository()
    service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider})
    input := validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription)

    first, err := service.Submit(context.Background(), input)
    if err != nil { t.Fatal(err) }
    second, err := service.Submit(context.Background(), input)
    if err != nil { t.Fatal(err) }
    if first.ID != second.ID || provider.submitCalls != 1 {
        t.Fatalf("jobs = %s/%s, submit calls = %d", first.ID, second.ID, provider.submitCalls)
    }
}

func TestDuplicateTerminalCallbackIsConsumedOnce(t *testing.T) {
    service, repo := newCallbackService(t)
    job := insertSubmittedJob(t, repo)
    snapshot := succeededTranscriptionSnapshot(job)
    if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil { t.Fatal(err) }
    if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil { t.Fatal(err) }
    _, ok, err := service.ConsumeTerminal(context.Background(), job.ID)
    if err != nil || !ok { t.Fatalf("first consume = %v, %v", ok, err) }
    _, ok, err = service.ConsumeTerminal(context.Background(), job.ID)
    if err != nil || ok { t.Fatalf("second consume = %v, %v", ok, err) }
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/providerjobs -run 'TestSubmit|TestDuplicateTerminal' -v`

Expected: FAIL because `Service` does not exist.

- [ ] **Step 3: Implement submit and refresh**

Define:

```go
type SubmitInput struct {
    ProjectID      uuid.UUID
    WorkflowRunID  uuid.UUID
    WorkflowNode   string
    ProviderKind   providers.Kind
    TaskType       providers.TaskType
    Input          json.RawMessage
    ResourceURLs   []string
    CallbackBaseURL string
    Deadline       time.Time
}

func (s *Service) Submit(ctx context.Context, input SubmitInput) (Job, error)
func (s *Service) Refresh(ctx context.Context, jobID uuid.UUID) (Job, error)
func (s *Service) ApplyCallback(ctx context.Context, jobID uuid.UUID, snapshot providers.Snapshot) error
func (s *Service) ConsumeTerminal(ctx context.Context, jobID uuid.UUID) (Outcome, bool, error)
```

The idempotency key is `runID:nodeName`. `Submit` persists before remote I/O, calls the correct registry entry only for a newly pending job, records a submit attempt, and stores the returned external ID. `Refresh` skips remote I/O for terminal jobs, otherwise calls `Status`, stores `snapshot.Raw` in MinIO, normalizes successful output with `providers.Normalize`, records an attempt, and applies the state.

Map `*providers.RemoteError` status 429 and 5xx to `retryable_failed`; map other 4xx to `permanently_failed`. A retryable failure records the attempt and job state but remains pollable; a later processing or success snapshot is valid. When the persisted deadline passes, `Refresh` applies `timed_out`. Do not automatically advance workflow state in this service.

Before `ApplyCallback` persists a snapshot, set `snapshot.Raw` to an immutable copy of the exact callback body. Before `MarkSubmitted`, archive `JobRef.Raw` as the submit attempt response.

- [ ] **Step 4: Write the failing callback signature tests**

```go
func TestCallbackRejectsExpiredTimestamp(t *testing.T) {
    verifier := NewCallbackVerifier([]byte("test-secret"), 5*time.Minute, func() time.Time {
        return time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
    })
    body := []byte(`{"request_id":"r","provider_job_id":"p","state":"processing"}`)
    timestamp := "2026-07-30T11:50:00Z"
    signature := signTestCallback([]byte("test-secret"), timestamp, body)
    if !errors.Is(verifier.Verify(timestamp, signature, body), ErrCallbackExpired) {
        t.Fatal("expired callback was accepted")
    }
}
```

- [ ] **Step 5: Implement HMAC verification and callback handler**

`CallbackVerifier.Verify` computes lowercase hex HMAC-SHA256 over `timestamp + "." + body`, compares with `hmac.Equal`, parses RFC3339 timestamps, and enforces an absolute five-minute difference.

The handler must:

1. Read at most 10 MiB.
2. Verify headers `X-Sangyu-Timestamp` and `X-Sangyu-Signature` before JSON decoding.
3. Parse and validate path `kind` and UUID `jobID`.
4. Load the job and reject the callback when path `kind` differs from the persisted Provider kind.
5. Call `Service.ApplyCallback`.
6. Enqueue an immediate Provider poll task so the workflow completion path consumes the terminal result.
7. Return 202 for both the first and duplicate valid callback.

Register this route outside `/v1/staff`:

```go
router.Post("/v1/provider-callbacks/{kind}/{jobID}", callbackHandler.Handle)
```

Extend `httpapi.Dependencies` with `RegisterProviderRoutes func(chi.Router)` instead of importing `providerjobs` into `httpapi`.

- [ ] **Step 6: Run tests and commit**

```powershell
go test ./internal/providerjobs ./internal/httpapi -v
go test ./...
git add internal/providerjobs internal/httpapi/router.go
git commit -m "feat: manage provider job lifecycle"
```

Expected: PASS.

### Task 5: Suspend and resume workflow nodes around asynchronous Provider jobs

**Files:**
- Create: `internal/workflow/provider_processor.go`
- Create: `internal/workflow/provider_processor_test.go`
- Modify: `internal/workflow/worker.go`
- Modify: `internal/workflow/worker_test.go`
- Modify: `internal/workflow/tasks.go`
- Modify: `internal/workflow/repository.go`
- Modify: `internal/book/workflow_processor.go`
- Modify: `internal/book/workflow_processor_test.go`
- Modify: `internal/book/repository.go`
- Modify: `internal/book/repository_test.go`

**Interfaces:**
- Changes: `Processor.Process(context.Context, NodePayload) (ProcessResult, error)`.
- Produces: `Completed(json.RawMessage) ProcessResult` and `Waiting(uuid.UUID) ProcessResult`.
- Produces: `Worker.ProcessProviderPoll(context.Context, ProviderPollPayload) error`.
- Produces: `AsynqEnqueuer.EnqueueNode` and `EnqueueProviderPoll`.
- Consumes: `providerjobs.Service` through narrow `JobSubmitter`, `JobRefresher`, and `TerminalConsumer` interfaces.

- [ ] **Step 1: Write failing worker suspension tests**

Replace the old processor test doubles with `ProcessResult` and add:

```go
func TestPendingProviderJobLeavesNodeRunningAndQueuesPoll(t *testing.T) {
    repo := newMemoryNodeRepository()
    queue := &memoryQueue{}
    jobID := uuid.New()
    worker := NewWorker(repo, map[NodeName]Processor{
        NodeTranscribe: processorFunc(func(context.Context, NodePayload) (ProcessResult, error) {
            return Waiting(jobID), nil
        }),
    }, queue, nil, time.Second)
    payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}

    if err := worker.Process(context.Background(), payload); err != nil { t.Fatal(err) }
    if repo.states[nodeKey(payload)] != NodeRunning { t.Fatal("node did not stay running") }
    if len(queue.providerPolls) != 1 || queue.providerPolls[0].JobID != jobID {
        t.Fatalf("polls = %#v", queue.providerPolls)
    }
}

func TestConsumedProviderSuccessAdvancesWorkflowOnce(t *testing.T) {
    payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}
    next := NodePayload{RunID: payload.RunID, ProjectID: payload.ProjectID, Node: NodeUnderstandPhoto}
    repo := newMemoryNodeRepository()
    repo.next = &next
    queue := &memoryQueue{}
    jobs := &memoryProviderJobs{
        job: providerjobs.Job{ID: uuid.New(), State: providers.StateSucceeded},
        outcome: providerjobs.Outcome{
            ProjectID: payload.ProjectID, WorkflowRunID: payload.RunID, WorkflowNode: string(payload.Node),
            State: providers.StateSucceeded,
            Output: json.RawMessage(`{"segments":[{"source_ref":"audio#0-1"}]}`),
        },
        consumable: true,
    }
    worker := NewWorker(repo, nil, queue, jobs, time.Second)
    poll := ProviderPollPayload{JobID: jobs.job.ID}

    if err := worker.ProcessProviderPoll(context.Background(), poll); err != nil { t.Fatal(err) }
    if err := worker.ProcessProviderPoll(context.Background(), poll); err != nil { t.Fatal(err) }
    if repo.succeedCalls != 1 { t.Fatalf("succeed calls = %d", repo.succeedCalls) }
    if len(queue.nodePayloads) != 1 || queue.nodePayloads[0] != next {
        t.Fatalf("next nodes = %#v", queue.nodePayloads)
    }
}
```

Implement `memoryProviderJobs.Refresh` to return `job` and `ConsumeTerminal` to return `outcome, true` once before setting `consumable=false`. Extend `memoryNodeRepository.SucceedNode` to increment `succeedCalls`, and rename the queue's existing payload slice to `nodePayloads`.

- [ ] **Step 2: Run the worker tests and verify failure**

Run: `go test ./internal/workflow -run 'TestPendingProvider|TestConsumedProvider' -v`

Expected: FAIL because `ProcessResult` and Provider poll processing do not exist.

- [ ] **Step 3: Change Processor to explicit completed or waiting outcomes**

Add to `worker.go`:

```go
type ProcessResult struct {
    Output        json.RawMessage
    ProviderJobID uuid.UUID
}

func Completed(output json.RawMessage) ProcessResult { return ProcessResult{Output: output} }
func Waiting(jobID uuid.UUID) ProcessResult { return ProcessResult{ProviderJobID: jobID} }
func (r ProcessResult) IsWaiting() bool { return r.ProviderJobID != uuid.Nil }

type Processor interface {
    Process(context.Context, NodePayload) (ProcessResult, error)
}
```

After `ClaimNode`, `Worker.Process` calls the processor. A waiting result enqueues `ProviderPollPayload{JobID: result.ProviderJobID}` using the configured poll interval and returns without calling `SucceedNode` or `FailNode`. A completed result follows the current path.

Change `NewWorker` to `NewWorker(repo NodeRepository, processors map[NodeName]Processor, queue Enqueuer, jobs ProviderJobs, pollInterval time.Duration) *Worker`. Replace a non-positive poll interval with two seconds inside the constructor.

Adapt `book.WorkflowProcessor.Process` to return `workflow.Completed(encodedArtifact)`. Change `book.PostgresRepository.LoadManuscript` to decode the `write_book` node output directly into `book.Manuscript`; canonical Provider output must not retain the old `{project_id, output}` test wrapper.

- [ ] **Step 4: Add Provider poll task support**

Use these payloads and interfaces:

```go
const (
    TaskWorkflowNode = "workflow:node"
    TaskProviderPoll = "provider:poll"
)

type ProviderPollPayload struct {
    JobID uuid.UUID `json:"job_id"`
}

type Enqueuer interface {
    EnqueueNode(context.Context, NodePayload) error
    EnqueueProviderPoll(context.Context, ProviderPollPayload, time.Duration) error
}
```

`AsynqEnqueuer.EnqueueProviderPoll` uses `asynq.ProcessIn(delay)`, `asynq.MaxRetry(3)`, and a 30-second worker timeout. Do not assign a permanent Asynq task ID to poll deliveries because successive polls need separate queue entries.

`Worker.ProcessProviderPoll` must:

1. Call `Refresh` unless the persisted job is already terminal.
2. For submitted, processing, or retryable-failed jobs, enqueue another poll using the configured `ProviderPollInterval`.
3. Call `ConsumeTerminal`.
4. Return without side effects when `consumed=false`.
5. Convert the consumed outcome linkage into `NodePayload{RunID: outcome.WorkflowRunID, ProjectID: outcome.ProjectID, Node: NodeName(outcome.WorkflowNode)}`.
6. On success, call `SucceedNode` with normalized output and enqueue the next node.
7. On permanently failed, timed-out, or cancelled outcome, call `FailNode` with the persisted error code.

- [ ] **Step 5: Implement the workflow Provider processor**

Create `provider_processor.go`:

```go
type JobSubmitter interface {
    Submit(context.Context, providerjobs.SubmitInput) (providerjobs.Job, error)
}

type ProviderProcessor struct {
    submitter    JobSubmitter
    providerKind providers.Kind
    taskType     providers.TaskType
    buildInput   func(context.Context, NodePayload) (json.RawMessage, []string, error)
    callbackBase string
}

func (p *ProviderProcessor) Process(ctx context.Context, payload NodePayload) (ProcessResult, error) {
    input, resources, err := p.buildInput(ctx, payload)
    if err != nil { return ProcessResult{}, err }
    job, err := p.submitter.Submit(ctx, providerjobs.SubmitInput{
        ProjectID: payload.ProjectID, WorkflowRunID: payload.RunID, WorkflowNode: string(payload.Node),
        ProviderKind: p.providerKind, TaskType: p.taskType, Input: input, ResourceURLs: resources,
        CallbackBaseURL: strings.TrimRight(p.callbackBase, "/"),
        Deadline: time.Now().UTC().Add(10 * time.Minute),
    })
    if err != nil { return ProcessResult{}, err }
    return Waiting(job.ID), nil
}
```

`providerjobs.Service.Submit` builds the final callback URL as `{CallbackBaseURL}/v1/provider-callbacks/{kind}/{internalJobID}` after it persists the internal job and before remote submission. Test that Media input builders can return resource URLs while Knowledge and Agent builders return none.

- [ ] **Step 6: Update repository and queue call sites**

Rename all current `Enqueue` calls to `EnqueueNode`. Register both handlers in `cmd/worker` later in Task 7. Keep node state `running` while a Provider job is pending. Do not add a fifth workflow-node state.

- [ ] **Step 7: Run tests and commit**

```powershell
go test ./internal/workflow ./internal/book -v
go test ./...
git add internal/workflow internal/book/workflow_processor.go internal/book/workflow_processor_test.go
git commit -m "feat: resume workflows from provider jobs"
```

Expected: PASS.

### Task 6: Replace the Skill Runner mock with a multi-kind Mock Provider

**Files:**
- Create: `providers/mock/package.json`
- Create: `providers/mock/server.mjs`
- Create: `providers/mock/server.test.mjs`
- Create: `providers/mock/Dockerfile`
- Modify: `deploy/local/compose.yaml`
- Modify: `.dockerignore`
- Delete: `skills/mock-memoir/package.json`
- Delete: `skills/mock-memoir/server.mjs`
- Delete: `skills/mock-memoir/server.test.mjs`
- Delete: `skills/mock-memoir/Dockerfile`
- Delete: `contracts/skill-runner/v1/invocation.schema.json`
- Delete: `contracts/skill-runner/v1/result.schema.json`
- Delete: `test/fixtures/skill-invocation.json`
- Delete: `internal/skillrunner/client.go`
- Delete: `internal/skillrunner/client_test.go`
- Delete: `internal/skillrunner/types.go`

**Interfaces:**
- Consumes: canonical Provider HTTP protocol and task types from Task 1.
- Produces: deterministic `POST /v1/jobs`, `GET /v1/jobs/{id}`, and `POST /v1/jobs/{id}:cancel` on port 8090.
- Produces: test-only failure selection through request input field `mock_scenario`.

- [ ] **Step 1: Write failing Node tests for asynchronous behavior**

Create tests that submit a job, observe processing, then observe success:

```js
test('runs a canonical asynchronous provider job', async () => {
  const app = createApp({ completionDelayMs: 5 })
  await usingServer(app, async (baseURL) => {
    const submitted = await postJSON(`${baseURL}/v1/jobs`, validSubmission('audio_transcription'))
    assert.equal(submitted.status, 202)
    assert.equal(submitted.body.state, 'submitted')

    await new Promise((resolve) => setTimeout(resolve, 10))
    const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
    assert.equal(result.body.state, 'succeeded')
    assert.equal(result.body.output.segments[0].source_ref, 'audio-fixture#12-20')
  })
})
```

Add these failure, idempotency, and cancellation tests:

```js
test('exposes selected terminal and retry states', async () => {
  const app = createApp({ completionDelayMs: 1 })
  await usingServer(app, async (baseURL) => {
    for (const expected of ['retryable_failed', 'permanently_failed', 'timed_out']) {
      const request = validSubmission('audio_transcription')
      request.idempotency_key = `scenario-${expected}`
      request.input.mock_scenario = expected
      const submitted = await postJSON(`${baseURL}/v1/jobs`, request)
      await new Promise((resolve) => setTimeout(resolve, 5))
      const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
      assert.equal(result.body.state, expected)
    }
  })
})

test('reuses idempotent submissions and supports cancellation', async () => {
  const app = createApp({ completionDelayMs: 1000 })
  await usingServer(app, async (baseURL) => {
    const request = validSubmission('chapter_writer')
    const first = await postJSON(`${baseURL}/v1/jobs`, request)
    const second = await postJSON(`${baseURL}/v1/jobs`, request)
    assert.equal(first.body.provider_job_id, second.body.provider_job_id)
    const cancelled = await postJSON(`${baseURL}/v1/jobs/${first.body.provider_job_id}:cancel`, {})
    assert.equal(cancelled.status, 204)
    const result = await getJSON(`${baseURL}/v1/jobs/${first.body.provider_job_id}`)
    assert.equal(result.body.state, 'cancelled')
  })
})

test('can return malformed successful output for adapter validation tests', async () => {
  const app = createApp({ completionDelayMs: 1 })
  await usingServer(app, async (baseURL) => {
    const request = validSubmission('audio_transcription')
    request.input.mock_scenario = 'malformed_output'
    const submitted = await postJSON(`${baseURL}/v1/jobs`, request)
    await new Promise((resolve) => setTimeout(resolve, 5))
    const result = await getJSON(`${baseURL}/v1/jobs/${submitted.body.provider_job_id}`)
    assert.equal(result.body.state, 'succeeded')
    assert.deepEqual(result.body.output, { unexpected: true })
  })
})
```

- [ ] **Step 2: Run the tests and verify failure**

Run: `npm --prefix providers/mock test`

Expected: FAIL because `providers/mock` does not exist.

- [ ] **Step 3: Implement the in-memory Mock Provider**

The service must:

- Accept all task types defined in Task 1.
- Return the existing deterministic memoir fixture data for the foundation workflow.
- Reuse the same job for the same `idempotency_key`.
- Transition from submitted to processing to the selected terminal state.
- Never fetch `resource_urls` and never access host paths.
- Return `output` only for succeeded jobs.
- Use `mock_scenario` only in local tests.
- Keep jobs in memory because it is explicitly a test double.

For `chapter_writer`, return a direct `book.Manuscript` JSON shape with `title` and `chapters`; do not wrap it in a second `output` property. For `shared_memory_retrieval`, include a `K-` prefixed reference, source, region, year range, confidence, and license.

- [ ] **Step 4: Replace Compose service and test isolation**

Rename `mock-skill-runner` to `mock-provider`. Keep port 8090, read-only filesystem, `/tmp` tmpfs, `no-new-privileges`, 256 MiB memory limit, non-root image user, and no host mounts. Update `.dockerignore` from `skills/mock-memoir/node_modules` to `providers/mock/node_modules`.

- [ ] **Step 5: Remove the obsolete Skill Runner only after replacement tests pass**

Run before deletion:

```powershell
npm --prefix providers/mock test
go test ./internal/providers -v
```

Expected: PASS.

Then delete the exact old files listed in this task using `apply_patch`; do not retain aliases named Skill Runner.

- [ ] **Step 6: Run repository checks and commit**

```powershell
npm --prefix providers/mock test
go test ./...
docker compose -f deploy/local/compose.yaml up -d --build mock-provider
Invoke-RestMethod -Method Post -ContentType 'application/json' -Body (Get-Content -Raw test/fixtures/providers/submit-transcription.json) -Uri 'http://localhost:8090/v1/jobs'
git add -A providers contracts internal skills test/fixtures deploy/local/compose.yaml .dockerignore
git commit -m "refactor: replace Skill Runner with mock Providers"
```

Expected: Node and Go tests PASS; HTTP returns 202 with a non-empty `provider_job_id`; the commit records deletions and additions.

### Task 7: Wire Provider configuration and the foundation workflow

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `.env.example`
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`
- Modify: `internal/workflow/model.go`
- Modify: `internal/workflow/processors.go`
- Modify: `internal/workflow/processors_test.go`
- Modify: `internal/workflow/repository.go`
- Modify: `internal/workflow/repository_test.go`
- Modify: `internal/assets/model.go`
- Modify: `internal/assets/repository.go`
- Modify: `internal/assets/repository_test.go`
- Modify: `internal/assets/minio_store.go`
- Modify: `internal/assets/minio_store_test.go`
- Modify: `deploy/local/compose.yaml`

**Interfaces:**
- Consumes: all three Provider clients, Provider job service, poll queue, and callback handler.
- Produces: a seven-node Provider-backed workflow ending in local PDF rendering.
- Produces: configuration fields `MediaProviderURL`, `MediaProviderToken`, `KnowledgeProviderURL`, `KnowledgeProviderToken`, `AgentProviderURL`, `AgentProviderToken`, `ProviderAllowedHosts`, `ProviderCallbackBaseURL`, `ProviderCallbackSecret`, and `ProviderPollInterval`.

- [ ] **Step 1: Write failing configuration tests**

```go
func TestLoadRequiresProviderConfiguration(t *testing.T) {
    setRequiredBaseEnvironment(t)
    t.Setenv("MEDIA_PROVIDER_URL", "http://mock-provider:8090")
    t.Setenv("MEDIA_PROVIDER_TOKEN", "media-token")
    t.Setenv("KNOWLEDGE_PROVIDER_URL", "http://mock-provider:8090")
    t.Setenv("KNOWLEDGE_PROVIDER_TOKEN", "knowledge-token")
    t.Setenv("AGENT_PROVIDER_URL", "http://mock-provider:8090")
    t.Setenv("AGENT_PROVIDER_TOKEN", "agent-token")
    t.Setenv("PROVIDER_ALLOWED_HOSTS", "mock-provider:8090")
    t.Setenv("PROVIDER_CALLBACK_BASE_URL", "http://api:8080")
    t.Setenv("PROVIDER_CALLBACK_SECRET", "local-callback-secret")
    t.Setenv("PROVIDER_POLL_INTERVAL", "2s")
    cfg, err := Load()
    if err != nil { t.Fatal(err) }
    if cfg.AgentProviderURL != "http://mock-provider:8090" { t.Fatalf("cfg = %#v", cfg) }
}
```

Also assert that an empty callback secret returns a configuration error.

- [ ] **Step 2: Run the configuration tests and verify failure**

Run: `go test ./internal/config -run TestLoadRequiresProviderConfiguration -v`

Expected: FAIL because the fields do not exist.

- [ ] **Step 3: Add exact environment configuration**

Add these variables to `.env.example` and Compose:

```text
MEDIA_PROVIDER_URL=http://localhost:8090
MEDIA_PROVIDER_TOKEN=local-media-token
KNOWLEDGE_PROVIDER_URL=http://localhost:8090
KNOWLEDGE_PROVIDER_TOKEN=local-knowledge-token
AGENT_PROVIDER_URL=http://localhost:8090
AGENT_PROVIDER_TOKEN=local-agent-token
PROVIDER_ALLOWED_HOSTS=localhost:8090
PROVIDER_CALLBACK_BASE_URL=http://localhost:8080
PROVIDER_CALLBACK_SECRET=sangyu-local-callback-secret
PROVIDER_POLL_INTERVAL=2s
```

Inside Compose use host `mock-provider:8090` and callback base `http://api:8080`. Parse the host list by comma, trim whitespace, and reject an empty result.

- [ ] **Step 4: Replace deterministic AI processors with Provider mappings**

Extend the node sequence:

```go
const NodeRetrieveSharedMemory NodeName = "retrieve_shared_memory"

var NodeSequence = []NodeName{
    NodeTranscribe,
    NodeUnderstandPhoto,
    NodeBuildMemory,
    NodeRetrieveSharedMemory,
    NodePlanBook,
    NodeWriteBook,
    NodeRenderPDF,
}
```

Map nodes exactly:

```text
transcribe             -> media / audio_transcription
understand_photo       -> media / photo_understanding
build_memory           -> agent / timeline_builder
retrieve_shared_memory -> knowledge / shared_memory_retrieval
plan_book              -> agent / chapter_planner
write_book             -> agent / chapter_writer
render_pdf             -> local book.WorkflowProcessor
```

Replace `DeterministicProcessors` with `ProviderProcessors(submitter, inputBuilder, callbackBaseURL)`. Input builders may use deterministic project/run metadata in this boundary-refactor phase, but Media builders must obtain short-lived source URLs from an injected `AssetReader`; Agent and Knowledge builders must return an empty resource URL slice.

Add these asset-side interfaces:

```go
type SourceAssetRepository interface {
    ListUploadedByKind(context.Context, uuid.UUID, Kind) ([]Asset, error)
}

type SourceURLStore interface {
    PresignGet(context.Context, string, string, time.Duration) (*url.URL, error)
}

type SourceReader struct {
    repo   SourceAssetRepository
    store  SourceURLStore
    bucket string
}

func (r *SourceReader) URLs(ctx context.Context, projectID uuid.UUID, kind Kind) ([]string, error)
```

`URLs` lists only uploaded assets for the requested project and kind and signs each object for 15 minutes. The media input builders call it for audio or photo. Agent and Knowledge input builders have no dependency on `SourceReader` and return an empty resource URL slice.

Update the `LatestRun` node-order SQL `CASE` so `retrieve_shared_memory` sorts between `build_memory` and `plan_book`. Add a repository test asserting the seven returned node names exactly match `NodeSequence`.

- [ ] **Step 5: Compose API and Worker dependencies**

In both processes:

1. Construct three separately configured `providers.HTTPClient` values.
2. Build `providers.Registry`.
3. Construct `providerjobs.PostgresRepository` and `MinioRawStore`.
4. Construct `providerjobs.Service`.

In API, construct the callback verifier and handler and register Provider routes outside `/v1/staff`.

In Worker, register both handlers:

```go
mux.Handle(workflow.TaskWorkflowNode, workflow.NewAsynqHandler(worker))
mux.Handle(workflow.TaskProviderPoll, workflow.NewProviderPollAsynqHandler(worker))
```

Keep Chromium rendering local. Do not instantiate or reference `skillrunner`.

- [ ] **Step 6: Run focused and full tests**

```powershell
go test ./internal/config ./internal/providers ./internal/providerjobs ./internal/workflow ./internal/book -v
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 7: Commit composition**

```powershell
git add .env.example cmd internal/config internal/workflow deploy/local/compose.yaml
git commit -m "feat: run memoir workflow through external Providers"
```

### Task 8: Prove the Provider-backed vertical slice and update product language

**Files:**
- Modify: `test/integration/foundation_test.go`
- Modify: `scripts/vertical-slice.ps1`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-28-foundation-vertical-slice.md`

**Interfaces:**
- Consumes: the complete Provider-backed stack.
- Produces: one command that proves all three Provider kinds were used before PDF creation.
- Preserves: existing staff mini-program API behavior.

- [ ] **Step 1: Write the failing Provider audit assertion**

Extend the integration test with a direct test-only PostgreSQL query under `TEST_DATABASE_URL`; do not add a staff API endpoint in this refactor:

```go
func assertProviderKindsUsed(t *testing.T, runID string) {
    pool, err := pgxpool.New(context.Background(), os.Getenv("TEST_DATABASE_URL"))
    if err != nil { t.Fatal(err) }
    defer pool.Close()
    rows, err := pool.Query(context.Background(), `
        SELECT provider_kind, count(*)
        FROM provider_jobs
        WHERE workflow_run_id = $1
        GROUP BY provider_kind`, runID)
    if err != nil { t.Fatal(err) }
    defer rows.Close()
    found := map[string]int{}
    for rows.Next() {
        var kind string
        var count int
        if err := rows.Scan(&kind, &count); err != nil { t.Fatal(err) }
        found[kind] = count
    }
    for _, kind := range []string{"media", "knowledge", "agent"} {
        if found[kind] == 0 { t.Fatalf("provider kind %s was not used: %#v", kind, found) }
    }
}
```

Call it after workflow completion. Also assert every succeeded Provider job has non-empty `normalized_output` and `raw_response_object_key`.

- [ ] **Step 2: Run the integration test against the old stack and verify failure**

```powershell
$env:TEST_API_URL='http://localhost:8080'
$env:TEST_DATABASE_URL='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go test ./test/integration -run TestFoundationVerticalSlice -v -count=1
```

Expected before rebuilding the new stack: FAIL because Provider audit records are absent.

- [ ] **Step 3: Update the smoke gate**

Replace the old mock skill test with:

```powershell
Invoke-Native npm --prefix providers/mock test
```

Set both `TEST_API_URL` and `TEST_DATABASE_URL` before the integration test. Keep full image build, migration, health wait, Go tests, `go vet`, mini-program tests, and TypeScript checks.

- [ ] **Step 4: Update README and historical plan status**

README must state:

- Sangyu Record owns collection, storage, orchestration, validation, PDF, and delivery.
- Media, Knowledge, and Agent Providers are external asynchronous APIs.
- The local Mock Provider is a test double only.
- Exact Provider environment variables and callback security behavior.
- Exact local smoke command.

At the top of `2026-07-28-foundation-vertical-slice.md`, add a short supersession note linking the 2026-07-30 design and this plan. Do not rewrite historical task details.

- [ ] **Step 5: Run the complete acceptance gate**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
git diff --check
git status --short
```

Expected:

- All Go, Node, and TypeScript tests PASS.
- All Compose services are running; PostgreSQL, Redis, and MinIO are healthy.
- Provider audit contains `media`, `knowledge`, and `agent` jobs.
- Raw response object keys and normalized outputs both exist and are separate.
- Duplicate poll delivery does not duplicate node advancement.
- The project reaches `completed`.
- Downloaded bytes begin with `%PDF-`.
- `git diff --check` prints nothing.

- [ ] **Step 6: Commit acceptance and documentation**

```powershell
git add test/integration/foundation_test.go scripts/vertical-slice.ps1 Makefile README.md docs/superpowers/plans/2026-07-28-foundation-vertical-slice.md
git commit -m "test: verify external Provider workflow"
```

## Final Verification

Run from `D:\Sangyu-record`:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-prerequisites.ps1
go test ./...
go vet ./...
npm --prefix providers/mock test
npm --prefix miniapp test
npm --prefix miniapp run typecheck
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
git diff --check
git status --short --branch
```

Success requires:

- No `internal/skillrunner`, `skills/mock-memoir`, or `contracts/skill-runner` files remain.
- Three independent Provider interfaces and configured adapters exist.
- External work does not hold a Worker for the Provider processing duration.
- Callback and polling completion paths converge on one atomic terminal consumption.
- Knowledge and Agent Provider submissions contain no raw asset URLs.
- The existing mini-program workflow still reaches and opens the generated PDF.
- The working tree is clean after commits.
