# Sangyu Record Foundation Vertical Slice Implementation Plan

> **Status (2026-07-30):** The Skill Runner architecture in this historical plan is superseded by the external Provider [design](../specs/2026-07-30-external-provider-boundary-design.md) and [implementation plan](2026-07-30-provider-boundary-refactor.md). The original details below are retained as implementation history.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable staff-only vertical slice that creates an elder memoir project, generates a deterministic collection plan, uploads source assets, executes a durable mocked automation workflow, and downloads a basic source-linked PDF.

**Architecture:** Use a modular Go monolith for HTTP APIs and durable domain state, plus a separate Go worker consuming Redis-backed jobs. PostgreSQL is authoritative for projects and workflow state, MinIO stores immutable assets and generated files, and an HTTP Skill Runner contract allows later Python, Node.js, or executable Skills without moving business orchestration out of Go.

**Tech Stack:** Go 1.26.5, chi v5.3.1, pgx v5.10.0, Asynq v0.26.0, MinIO Go SDK v7.2.1, goose v3.27.3, PostgreSQL 17, Redis 7, MinIO, native WeChat Mini Program with TypeScript 7.0.2 and Vitest 4.1.10, Docker Compose.

## Global Constraints

- The core API, workflow orchestration, persistence, model gateway boundary, and workers are written in Go.
- Skills may use Python, Node.js, or native executables, but Go invokes them only through the versioned Skill Runner contract.
- The first slice uses deterministic fake AI outputs; real speech, vision, retrieval, and writing models are outside this plan.
- Original audio, photos, and staff notes are immutable evidence and cannot be overwritten by derived content.
- Personal facts must retain source asset and time-range metadata; shared-memory content cannot become a personal fact.
- Household projects are private by default; shared-memory contribution is outside this plan.
- Normal mocked projects complete automatically; invalid files and failed workflow nodes produce explicit errors.
- Use TDD for domain behavior and integration boundaries; each task ends with a focused test command and commit.
- On Windows, run commands in PowerShell from `D:\Sangyu-record`.

---

## Scope Boundary

This is the first of six implementation plans. It proves the system boundaries and end-to-end state flow with deterministic adapters.

Included:

- Local development dependencies and Go services.
- Staff project creation and deterministic collection-plan generation.
- Presigned uploads for audio and photos.
- Durable workflow jobs and idempotent state transitions.
- A versioned HTTP Skill Runner contract with a deterministic test server.
- A basic HTML manuscript and PDF artifact.
- Native mini-program pages for project creation, upload, status, and download.
- End-to-end tests against real PostgreSQL, Redis, and MinIO containers.

Deferred to later plans:

- Speech-to-text, diarization, photo understanding, OCR, and real model adapters.
- Personal-memory extraction, pgvector retrieval, and common-memory ingestion.
- Production Skills for memoir positioning, chapter planning, writing, fact checking, and privacy review.
- Formal authorization records, sensitive-content policy, print-grade templates, and publisher integration.
- Production authentication, staff assignment, billing, family accounts, and public deployment.

## File Structure

```text
cmd/
  api/main.go                         API process composition root
  worker/main.go                      Asynq worker composition root
internal/
  config/config.go                    Environment configuration
  httpapi/router.go                   HTTP middleware and route registration
  httpapi/response.go                 JSON success/error envelopes
  platform/postgres.go                pgx pool construction
  platform/redis.go                   Asynq client/server construction
  platform/objectstore.go             MinIO-backed object storage
  projects/model.go                   Project and collection-plan domain types
  projects/repository.go              PostgreSQL project persistence
  projects/service.go                 Project creation and state rules
  projects/handler.go                 Staff project HTTP endpoints
  assets/model.go                     Asset and upload domain types
  assets/repository.go                PostgreSQL asset persistence
  assets/service.go                   Upload initiation and completion
  assets/handler.go                   Asset HTTP endpoints
  workflow/model.go                   Workflow state and task payloads
  workflow/repository.go              Durable workflow persistence
  workflow/service.go                 Start and inspect workflow
  workflow/tasks.go                   Asynq task names and payload codecs
  workflow/worker.go                  Idempotent node handlers
  skillrunner/client.go               Versioned HTTP Skill Runner client
  skillrunner/types.go                Invocation and result contracts
  book/model.go                       Structured manuscript types
  book/renderer.go                    HTML and PDF rendering boundary
  book/template.go                    Embedded basic book template
  book/handler.go                     Artifact download endpoint
migrations/
  00001_foundation.sql                Foundation schema
miniapp/
  app.json                            Mini-program routes
  app.ts                              Application entry
  app.wxss                            Global restrained workbench styles
  env.ts                              API base URL
  services/api.ts                     Typed API client
  services/api.test.ts                API client tests
  pages/projects/*                    Project list
  pages/create/*                      Project creation
  pages/project/*                     Collection plan and workflow status
  pages/upload/*                      Audio/photo upload
  pages/result/*                      Result download
test/integration/
  foundation_test.go                  Container-backed vertical-slice test
test/fixtures/
  interview.wav                       Small deterministic audio fixture
  family-photo.jpg                    Small deterministic image fixture
deploy/local/
  compose.yaml                        PostgreSQL, Redis, MinIO, API, worker, Chromium
  Dockerfile.api                      Go API image
  Dockerfile.worker                   Go worker image
  Dockerfile.chromium                 Chromium PDF service image
scripts/
  verify-prerequisites.ps1            Local tool checks
  vertical-slice.ps1                  End-to-end smoke test
go.mod
go.sum
Makefile
.env.example
.gitignore
```

### Task 1: Bootstrap the Go services and health contract

**Files:**
- Create: `go.mod`
- Create: `cmd/api/main.go`
- Create: `cmd/worker/main.go`
- Create: `internal/config/config.go`
- Create: `internal/httpapi/router.go`
- Create: `internal/httpapi/response.go`
- Create: `internal/httpapi/router_test.go`
- Create: `scripts/verify-prerequisites.ps1`
- Create: `.gitignore`
- Create: `.env.example`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)`.
- Produces: `httpapi.NewRouter(httpapi.Dependencies) http.Handler`.
- Produces: `GET /healthz -> 200 {"status":"ok"}`.

- [ ] **Step 1: Add the prerequisite verifier and run it before creating the module**

```powershell
$required = @('go', 'node', 'npm', 'docker')
$missing = @()
foreach ($command in $required) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        $missing += $command
    }
}
if ($missing.Count -gt 0) {
    Write-Error ('Missing required commands: ' + ($missing -join ', '))
    exit 1
}

$goVersion = (& go version)
if ($goVersion -notmatch 'go1\.26\.') {
    Write-Error "Go 1.26.x is required; found: $goVersion"
    exit 1
}

docker info | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Error 'Docker Desktop must be running.'
    exit 1
}

Write-Output 'Prerequisites verified.'
```

Run: `powershell -ExecutionPolicy Bypass -File scripts/verify-prerequisites.ps1`

Expected now: FAIL with missing `go` and unavailable Docker. Install Go 1.26.x and start Docker Desktop before continuing implementation.

- [ ] **Step 2: Write the failing router test**

```go
package httpapi_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/nevermore222/sangyu-record/internal/httpapi"
)

func TestHealth(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()
    httpapi.NewRouter(httpapi.Dependencies{}).ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
    }
    if got := rec.Body.String(); got != "{\"status\":\"ok\"}\n" {
        t.Fatalf("body = %q", got)
    }
}
```

- [ ] **Step 3: Initialize the module and verify the test fails**

Run:

```powershell
go mod init github.com/nevermore222/sangyu-record
go get github.com/go-chi/chi/v5@v5.3.1
go test ./internal/httpapi -run TestHealth -v
```

Expected: FAIL because `internal/httpapi` does not exist.

- [ ] **Step 4: Implement configuration and the health router**

```go
// internal/config/config.go
package config

import (
    "errors"
    "os"
)

type Config struct {
    HTTPAddress string
    DatabaseURL string
    RedisURL    string
    S3Endpoint  string
    S3AccessKey string
    S3SecretKey string
    S3Bucket    string
}

func Load() (Config, error) {
    cfg := Config{
        HTTPAddress: envOr("HTTP_ADDRESS", ":8080"),
        DatabaseURL: os.Getenv("DATABASE_URL"),
        RedisURL:    os.Getenv("REDIS_URL"),
        S3Endpoint:  os.Getenv("S3_ENDPOINT"),
        S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
        S3SecretKey: os.Getenv("S3_SECRET_KEY"),
        S3Bucket:    envOr("S3_BUCKET", "sangyu-private"),
    }
    if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.S3Endpoint == "" {
        return Config{}, errors.New("DATABASE_URL, REDIS_URL, and S3_ENDPOINT are required")
    }
    return cfg, nil
}

func envOr(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

```go
// internal/httpapi/response.go
package httpapi

import (
    "encoding/json"
    "net/http"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(value)
}
```

```go
// internal/httpapi/router.go
package httpapi

import (
    "net/http"

    "github.com/go-chi/chi/v5"
)

type Dependencies struct{}

func NewRouter(_ Dependencies) http.Handler {
    router := chi.NewRouter()
    router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
    })
    return router
}
```

Implement `cmd/api/main.go` to load config, create the router, and run `http.Server` with read-header and idle timeouts. Implement `cmd/worker/main.go` as a process that loads config and exits with a clear log until Task 5 supplies the worker server.

- [ ] **Step 5: Run focused and repository tests**

Run:

```powershell
go test ./internal/httpapi -run TestHealth -v
go test ./...
go vet ./...
```

Expected: all commands PASS.

- [ ] **Step 6: Commit the bootstrap**

```powershell
git add go.mod go.sum cmd internal/config internal/httpapi scripts/verify-prerequisites.ps1 .gitignore .env.example
git commit -m "build: bootstrap Go API and worker"
```

### Task 2: Add local infrastructure and the foundation schema

**Files:**
- Create: `deploy/local/compose.yaml`
- Create: `deploy/local/Dockerfile.api`
- Create: `deploy/local/Dockerfile.worker`
- Create: `migrations/00001_foundation.sql`
- Create: `internal/platform/postgres.go`
- Create: `internal/platform/redis.go`
- Create: `internal/platform/objectstore.go`
- Create: `internal/platform/platform_test.go`
- Create: `Makefile`

**Interfaces:**
- Produces: `platform.OpenPostgres(ctx, databaseURL) (*pgxpool.Pool, error)`.
- Produces: `platform.NewAsynqClient(redisURL) (*asynq.Client, error)`.
- Produces: `platform.NewObjectStore(config) (*minio.Client, error)`.
- Produces tables: `projects`, `collection_plan_items`, `assets`, `workflow_runs`, `workflow_nodes`, `artifacts`.

- [ ] **Step 1: Write the container-backed connection test**

```go
package platform_test

import (
    "context"
    "os"
    "testing"

    "github.com/nevermore222/sangyu-record/internal/platform"
)

func TestPostgresConnection(t *testing.T) {
    url := os.Getenv("TEST_DATABASE_URL")
    if url == "" {
        t.Skip("TEST_DATABASE_URL is not set")
    }
    pool, err := platform.OpenPostgres(context.Background(), url)
    if err != nil {
        t.Fatal(err)
    }
    defer pool.Close()
    if err := pool.Ping(context.Background()); err != nil {
        t.Fatal(err)
    }
}
```

- [ ] **Step 2: Add Compose services and verify readiness**

Create Compose services with exact names `postgres`, `redis`, `minio`, and `minio-init`; expose PostgreSQL on `5432`, Redis on `6379`, MinIO API on `9000`, and MinIO console on `9001`. Use named volumes and health checks. The initialization service must create the private bucket `sangyu-private` without making it public.

Run:

```powershell
docker compose -f deploy/local/compose.yaml up -d postgres redis minio minio-init
docker compose -f deploy/local/compose.yaml ps
```

Expected: `postgres`, `redis`, and `minio` report healthy; `minio-init` exits with code 0.

- [ ] **Step 3: Add the migration**

The migration must define UUID primary keys, UTC timestamps, project-scoped foreign keys, immutable object keys, unique workflow node keys `(run_id, node_name)`, and explicit check constraints for states. Use these initial state values:

```sql
CREATE TYPE project_state AS ENUM (
  'collecting', 'processing', 'needs_material', 'generating',
  'quality_check', 'exception', 'pdf_rendering', 'completed'
);

CREATE TYPE asset_kind AS ENUM ('audio', 'photo', 'staff_note');
CREATE TYPE asset_state AS ENUM ('pending_upload', 'uploaded', 'verified', 'rejected');
CREATE TYPE node_state AS ENUM ('queued', 'running', 'succeeded', 'failed');
```

Include reversible `-- +goose Up` and `-- +goose Down` sections in `migrations/00001_foundation.sql`.

- [ ] **Step 4: Implement platform constructors**

Use `pgxpool.New`, `asynq.ParseRedisURI`, and `minio.New` with explicit TLS configuration. Do not hide retries in constructors; callers own process startup retry behavior.

Run:

```powershell
go get github.com/jackc/pgx/v5@v5.10.0
go get github.com/hibiken/asynq@v0.26.0
go get github.com/minio/minio-go/v7@v7.2.1
go get github.com/pressly/goose/v3@v3.27.3
go test ./internal/platform -v
```

Expected: PASS with `TEST_DATABASE_URL` set; otherwise the integration case is explicitly SKIP.

- [ ] **Step 5: Apply and verify the migration**

Run:

```powershell
$env:GOOSE_DRIVER='postgres'
$env:GOOSE_DBSTRING='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations status
```

Expected: migration `00001_foundation.sql` is `Applied`.

- [ ] **Step 6: Commit infrastructure**

```powershell
git add deploy/local migrations internal/platform Makefile go.mod go.sum
git commit -m "build: add local persistence infrastructure"
```

### Task 3: Implement project creation and collection plans

**Files:**
- Create: `internal/projects/model.go`
- Create: `internal/projects/repository.go`
- Create: `internal/projects/service.go`
- Create: `internal/projects/handler.go`
- Create: `internal/projects/service_test.go`
- Modify: `internal/httpapi/router.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: `*pgxpool.Pool` from Task 2.
- Produces: `projects.Service.Create(ctx, CreateInput) (Project, error)`.
- Produces: `projects.Service.Get(ctx, projectID uuid.UUID) (ProjectDetail, error)`.
- Produces: `POST /v1/staff/projects` and `GET /v1/staff/projects/{projectID}`.

- [ ] **Step 1: Write the failing service test**

```go
func TestCreateGeneratesFirstCollectionPlan(t *testing.T) {
    repo := newMemoryRepository()
    service := projects.NewService(repo, projects.DeterministicPlanner{})

    created, err := service.Create(context.Background(), projects.CreateInput{
        DisplayName:       "林奶奶",
        BirthYear:         1948,
        BirthPlace:        "江苏苏州",
        LongTermResidence: "江苏苏州",
        PrimaryOccupation: "纺织工人",
        TargetEdition:     "standard",
    })
    if err != nil {
        t.Fatal(err)
    }
    if created.State != projects.StateCollecting {
        t.Fatalf("state = %s", created.State)
    }
    if len(created.CollectionPlan) < 5 {
        t.Fatalf("plan items = %d, want at least 5", len(created.CollectionPlan))
    }
    if created.CollectionPlan[0].Status != projects.PlanPending {
        t.Fatalf("first item status = %s", created.CollectionPlan[0].Status)
    }
}
```

- [ ] **Step 2: Run the test and verify failure**

Run: `go test ./internal/projects -run TestCreateGeneratesFirstCollectionPlan -v`

Expected: FAIL because the package and service do not exist.

- [ ] **Step 3: Implement domain types and deterministic planning**

Define `ProjectState`, `PlanItemStatus`, `CreateInput`, `Project`, `PlanItem`, `ProjectDetail`, `Repository`, and `Planner`. `DeterministicPlanner` must always create required items for childhood, education, work, family, turning points, and photos, plus a work-specific question when `PrimaryOccupation` is present.

```go
type Planner interface {
    BuildInitialPlan(CreateInput) []PlanItem
}

type Repository interface {
    Create(context.Context, Project) error
    Get(context.Context, uuid.UUID) (ProjectDetail, error)
}

type Service struct {
    repo    Repository
    planner Planner
}
```

Validate birth year between 1900 and the current year, non-empty display name, and target edition in `brief`, `standard`, or `long`. Return typed validation and not-found errors for HTTP mapping.

- [ ] **Step 4: Implement PostgreSQL persistence and handlers**

Use a transaction to insert the project and all collection-plan items. The POST handler returns `201`; invalid input returns `422`; unknown IDs return `404`. Register routes beneath `/v1/staff/projects`.

Example request:

```json
{
  "display_name": "林奶奶",
  "birth_year": 1948,
  "birth_place": "江苏苏州",
  "long_term_residence": "江苏苏州",
  "primary_occupation": "纺织工人",
  "target_edition": "standard"
}
```

- [ ] **Step 5: Run tests and a real API check**

Run:

```powershell
go test ./internal/projects ./internal/httpapi -v
go test ./...
```

Start the API, POST the example request, then GET the returned ID. Expected: `201`, state `collecting`, and at least six pending plan items.

- [ ] **Step 6: Commit project creation**

```powershell
git add cmd/api internal/projects internal/httpapi
git commit -m "feat: create memoir projects and collection plans"
```

### Task 4: Add immutable asset uploads

**Files:**
- Create: `internal/assets/model.go`
- Create: `internal/assets/repository.go`
- Create: `internal/assets/service.go`
- Create: `internal/assets/handler.go`
- Create: `internal/assets/service_test.go`
- Create: `test/fixtures/interview.wav`
- Create: `test/fixtures/family-photo.jpg`
- Modify: `internal/platform/objectstore.go`
- Modify: `internal/httpapi/router.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: project IDs from Task 3 and MinIO client from Task 2.
- Produces: `assets.Service.Initiate(ctx, InitiateInput) (UploadTicket, error)`.
- Produces: `assets.Service.Complete(ctx, assetID uuid.UUID, sha256 string) (Asset, error)`.
- Produces: `POST /v1/staff/projects/{projectID}/assets:initiate` and `POST /v1/staff/assets/{assetID}:complete`.

- [ ] **Step 1: Write failing validation and immutability tests**

```go
func TestInitiateRejectsUnsupportedContentType(t *testing.T) {
    service := newTestService()
    _, err := service.Initiate(context.Background(), assets.InitiateInput{
        ProjectID:   uuid.New(),
        Kind:        assets.KindAudio,
        Filename:    "interview.exe",
        ContentType: "application/octet-stream",
        SizeBytes:   100,
    })
    if !errors.Is(err, assets.ErrUnsupportedContentType) {
        t.Fatalf("err = %v", err)
    }
}

func TestCompleteDoesNotReplaceObjectKey(t *testing.T) {
    service, repo := newTestServiceWithPendingAsset()
    first, err := service.Complete(context.Background(), repo.asset.ID, strings.Repeat("a", 64))
    if err != nil {
        t.Fatal(err)
    }
    second, err := service.Complete(context.Background(), repo.asset.ID, strings.Repeat("a", 64))
    if err != nil {
        t.Fatal(err)
    }
    if first.ObjectKey != second.ObjectKey {
        t.Fatal("completion changed immutable object key")
    }
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./internal/assets -v`

Expected: FAIL because the asset service does not exist.

- [ ] **Step 3: Implement upload initiation**

Accept `audio/mpeg`, `audio/mp4`, `audio/wav`, `image/jpeg`, and `image/png`. Limit individual audio files to 2 GiB and photos to 30 MiB. Generate object keys as `projects/{projectID}/source/{assetID}/{sanitizedFilename}`. Return a 30-minute presigned PUT URL with required content type.

```go
type UploadTicket struct {
    AssetID  uuid.UUID `json:"asset_id"`
    UploadURL string    `json:"upload_url"`
    ExpiresAt time.Time `json:"expires_at"`
}
```

- [ ] **Step 4: Implement completion verification**

On completion, stat the object, compare size and content type with the pending record, store the client-calculated SHA-256, and mark the asset uploaded. Repeated completion with the same hash is idempotent; a different hash returns `409 Conflict` and never replaces the object key.

- [ ] **Step 5: Run tests and MinIO integration check**

Run:

```powershell
go test ./internal/assets -v
go test ./...
```

Upload `test/fixtures/interview.wav` through the presigned URL, complete it, and GET the project. Expected: the asset is `uploaded` with an immutable object key and SHA-256.

- [ ] **Step 6: Commit asset uploads**

```powershell
git add internal/assets internal/platform/objectstore.go internal/httpapi/router.go cmd/api/main.go test/fixtures
git commit -m "feat: upload immutable memoir source assets"
```

### Task 5: Implement the durable mocked automation workflow

**Files:**
- Create: `internal/workflow/model.go`
- Create: `internal/workflow/repository.go`
- Create: `internal/workflow/service.go`
- Create: `internal/workflow/tasks.go`
- Create: `internal/workflow/worker.go`
- Create: `internal/workflow/worker_test.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Consumes: a project with at least one uploaded audio asset and one uploaded photo.
- Produces: `workflow.Service.Start(ctx, projectID) (Run, error)`.
- Produces: node sequence `transcribe -> understand_photo -> build_memory -> plan_book -> write_book -> render_pdf`.
- Produces: `POST /v1/staff/projects/{projectID}/workflow:start` and `GET /v1/staff/projects/{projectID}/workflow`.

- [ ] **Step 1: Write the failing idempotency test**

```go
func TestNodeSuccessIsIdempotent(t *testing.T) {
    repo := newMemoryWorkflowRepository()
    worker := workflow.NewWorker(repo, deterministicProcessors())
    payload := workflow.NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: workflow.NodeTranscribe}

    if err := worker.Process(context.Background(), payload); err != nil {
        t.Fatal(err)
    }
    if err := worker.Process(context.Background(), payload); err != nil {
        t.Fatal(err)
    }
    if got := repo.successCount(payload.RunID, payload.Node); got != 1 {
        t.Fatalf("success transitions = %d, want 1", got)
    }
}
```

- [ ] **Step 2: Verify the test fails**

Run: `go test ./internal/workflow -run TestNodeSuccessIsIdempotent -v`

Expected: FAIL because workflow types are undefined.

- [ ] **Step 3: Implement state and queue contracts**

```go
const TaskWorkflowNode = "workflow:node"

type NodeName string

const (
    NodeTranscribe     NodeName = "transcribe"
    NodeUnderstandPhoto NodeName = "understand_photo"
    NodeBuildMemory    NodeName = "build_memory"
    NodePlanBook       NodeName = "plan_book"
    NodeWriteBook      NodeName = "write_book"
    NodeRenderPDF      NodeName = "render_pdf"
)

type NodePayload struct {
    RunID     uuid.UUID `json:"run_id"`
    ProjectID uuid.UUID `json:"project_id"`
    Node      NodeName  `json:"node"`
}
```

The repository must acquire a row lock before changing a node from queued to running, treat succeeded nodes as no-ops, save output JSON, and enqueue only the next node after committing success.

- [ ] **Step 4: Implement deterministic processors**

The mocked processors must produce stable fixture data:

- `transcribe`: a timestamped transcript referencing the uploaded audio asset.
- `understand_photo`: a candidate description explicitly marked `inferred`.
- `build_memory`: one personal memory sourced only from transcript timestamps.
- `plan_book`: a one-chapter standard-edition plan.
- `write_book`: a title, introduction, and chapter that do not convert photo inference into fact.
- `render_pdf`: delegates to Task 7's renderer; until Task 7 lands, return a typed `ErrRendererUnavailable` and leave the node failed.

- [ ] **Step 5: Wire Asynq and workflow endpoints**

The API enqueues the first node with a deterministic task ID `{runID}:transcribe`. The worker registers one handler for `workflow:node`, decodes the payload, executes the node, persists output, and enqueues the next node. Configure a maximum of three retries and a 10-minute timeout per mocked node.

- [ ] **Step 6: Test through the expected pre-render failure**

Run:

```powershell
go test ./internal/workflow -v
go test ./...
```

Start API and worker, start a workflow, and poll its status. Expected before Task 7: nodes through `write_book` succeed; `render_pdf` fails with `renderer_unavailable`; the project enters `exception` with prior outputs preserved.

- [ ] **Step 7: Commit durable workflow orchestration**

```powershell
git add internal/workflow cmd/api/main.go cmd/worker/main.go internal/httpapi/router.go
git commit -m "feat: orchestrate durable memoir workflow"
```

### Task 6: Define and exercise the multi-language Skill Runner contract

**Files:**
- Create: `internal/skillrunner/types.go`
- Create: `internal/skillrunner/client.go`
- Create: `internal/skillrunner/client_test.go`
- Create: `contracts/skill-runner/v1/invocation.schema.json`
- Create: `contracts/skill-runner/v1/result.schema.json`
- Create: `skills/mock-memoir/package.json`
- Create: `skills/mock-memoir/server.mjs`
- Create: `skills/mock-memoir/server.test.mjs`
- Create: `test/fixtures/skill-invocation.json`
- Modify: `deploy/local/compose.yaml`

**Interfaces:**
- Consumes: versioned invocation JSON from the Go workflow.
- Produces: `skillrunner.Client.Run(ctx, Invocation) (Result, error)`.
- Produces: `POST /v1/invocations` on an isolated runner.

- [ ] **Step 1: Write the failing Go client contract test**

```go
func TestClientRunsVersionedInvocation(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/v1/invocations" {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = io.WriteString(w, `{"invocation_id":"inv-1","status":"succeeded","output":{"title":"岁月留声"},"evidence_refs":["asset-1#0-12"]}`)
    }))
    defer server.Close()

    client := skillrunner.NewClient(server.URL, server.Client())
    result, err := client.Run(context.Background(), skillrunner.Invocation{
        InvocationID: "inv-1",
        ContractVersion: "1.0",
        Skill: skillrunner.SkillRef{Name: "mock-memoir", Version: "0.1.0"},
        Input: json.RawMessage(`{"project_id":"project-1"}`),
    })
    if err != nil {
        t.Fatal(err)
    }
    if result.Status != skillrunner.StatusSucceeded || len(result.EvidenceRefs) != 1 {
        t.Fatalf("result = %#v", result)
    }
}
```

- [ ] **Step 2: Define the JSON schemas**

The invocation schema requires `invocation_id`, `contract_version`, `skill.name`, `skill.version`, `input`, `allowed_resources`, and `deadline`. The result schema requires `invocation_id`, `status`, `output`, `evidence_refs`, `warnings`, `metrics`, and a nullable structured error. Both schemas set `additionalProperties` to false at the envelope level.

- [ ] **Step 3: Implement the Go client**

Use a caller-provided `http.Client`, POST JSON, cap response bodies at 10 MiB, require matching invocation IDs, and map non-2xx responses to `RemoteError`. The client must not retry; retry policy belongs to the workflow node.

- [ ] **Step 4: Implement the isolated Node test Skill**

The Node service accepts only the `mock-memoir` Skill at version `0.1.0`, returns deterministic JSON, never reads host paths, and listens on port `8090`. Run it in Compose with a read-only filesystem, no host mounts, a non-root user, memory limit, and only the internal network.

- [ ] **Step 5: Run contract tests**

Run:

```powershell
go test ./internal/skillrunner -v
npm --prefix skills/mock-memoir test
docker compose -f deploy/local/compose.yaml up -d mock-skill-runner
Invoke-RestMethod -Method Post -ContentType 'application/json' -Body (Get-Content -Raw test/fixtures/skill-invocation.json) -Uri 'http://localhost:8090/v1/invocations'
```

Expected: Go and Node tests PASS; HTTP result has `status=succeeded` and at least one evidence reference.

- [ ] **Step 6: Commit the Skill Runner boundary**

```powershell
git add internal/skillrunner contracts skills/mock-memoir deploy/local/compose.yaml test/fixtures/skill-invocation.json
git commit -m "feat: define isolated Skill Runner contract"
```

### Task 7: Render a basic source-linked PDF

**Files:**
- Create: `internal/book/model.go`
- Create: `internal/book/template.go`
- Create: `internal/book/renderer.go`
- Create: `internal/book/renderer_test.go`
- Create: `internal/book/handler.go`
- Create: `deploy/local/Dockerfile.chromium`
- Modify: `internal/workflow/worker.go`
- Modify: `internal/httpapi/router.go`
- Modify: `deploy/local/compose.yaml`

**Interfaces:**
- Consumes: deterministic manuscript JSON from `write_book`.
- Produces: `book.Renderer.Render(ctx, Manuscript) (Artifact, error)`.
- Produces: immutable PDF object `projects/{projectID}/artifacts/{versionID}/memoir.pdf`.
- Produces: `GET /v1/staff/projects/{projectID}/artifacts/latest`.

- [ ] **Step 1: Write the failing manuscript rendering test**

```go
func TestHTMLIncludesEvidenceMap(t *testing.T) {
    manuscript := book.Manuscript{
        Title: "岁月留声",
        Chapters: []book.Chapter{{
            Title: "纺织厂的日子",
            Paragraphs: []book.Paragraph{{
                Text: "1978年，她进入了当地纺织厂。",
                EvidenceRefs: []string{"asset-audio#12-20"},
            }},
        }},
    }
    html, err := book.RenderHTML(manuscript)
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(html, "asset-audio#12-20") {
        t.Fatal("rendered HTML omitted internal evidence map")
    }
}
```

- [ ] **Step 2: Verify the test fails**

Run: `go test ./internal/book -run TestHTMLIncludesEvidenceMap -v`

Expected: FAIL because the book package does not exist.

- [ ] **Step 3: Implement manuscript and HTML rendering**

Define `Manuscript`, `Chapter`, `Paragraph`, `Photo`, and `Artifact`. Use an embedded `html/template` with A5 page rules, Chinese font fallbacks, page breaks before chapters, bounded images, page numbers, and an internal evidence appendix. Escape all model-generated strings through `html/template`; do not mark them as trusted HTML.

- [ ] **Step 4: Implement Chromium PDF rendering**

Expose an internal Chromium render endpoint that accepts HTML and returns PDF bytes. The Go renderer sends a 30-second request, validates the `%PDF-` header and non-zero size, uploads to MinIO, and inserts the artifact record only after successful upload.

- [ ] **Step 5: Complete and verify the workflow**

Replace `ErrRendererUnavailable` with the real renderer. Start a new project workflow and poll until completion.

Run:

```powershell
go test ./internal/book ./internal/workflow -v
go test ./...
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
```

Expected: workflow reaches `completed`; downloaded file begins with `%PDF-` and contains at least one page.

- [ ] **Step 6: Commit PDF output**

```powershell
git add internal/book internal/workflow/worker.go internal/httpapi/router.go deploy/local
git commit -m "feat: render source-linked memoir PDF"
```

### Task 8: Add the staff mini-program vertical slice

**Files:**
- Create: `miniapp/package.json`
- Create: `miniapp/tsconfig.json`
- Create: `miniapp/project.config.json`
- Create: `miniapp/app.json`
- Create: `miniapp/app.ts`
- Create: `miniapp/app.wxss`
- Create: `miniapp/env.ts`
- Create: `miniapp/services/api.ts`
- Create: `miniapp/services/api.test.ts`
- Create: `miniapp/pages/projects/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/create/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/project/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/upload/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/result/index.{json,ts,wxml,wxss}`

**Interfaces:**
- Consumes: all `/v1/staff` endpoints from Tasks 3, 4, 5, and 7.
- Produces: a staff workflow from project creation through result download.

- [ ] **Step 1: Write the failing API-client test**

```ts
import { describe, expect, it, vi } from 'vitest'
import { createAPI } from './api'

describe('API client', () => {
  it('creates a project and returns the collection plan', async () => {
    const request = vi.fn().mockResolvedValue({
      statusCode: 201,
      data: { id: 'project-1', state: 'collecting', collection_plan: [{ id: 'plan-1' }] },
    })
    const api = createAPI({ baseURL: 'http://localhost:8080', request })
    const project = await api.createProject({
      display_name: '林奶奶',
      birth_year: 1948,
      birth_place: '江苏苏州',
      long_term_residence: '江苏苏州',
      primary_occupation: '纺织工人',
      target_edition: 'standard',
    })
    expect(project.collection_plan).toHaveLength(1)
    expect(request).toHaveBeenCalledWith(expect.objectContaining({ method: 'POST' }))
  })
})
```

- [ ] **Step 2: Initialize tooling and verify failure**

Run:

```powershell
npm --prefix miniapp install --save-dev typescript@7.0.2 vitest@4.1.10 miniprogram-api-typings@5.2.2
npm --prefix miniapp test
```

Expected: FAIL because `services/api.ts` does not exist.

- [ ] **Step 3: Implement the typed API client**

Wrap `wx.request`, `wx.uploadFile`, and `wx.downloadFile`; reject non-2xx status codes with a typed `APIError`; use the initiation ticket before uploading; call completion only after successful upload. Keep API base URL in `env.ts`, not page files.

- [ ] **Step 4: Implement stable staff pages**

Use a quiet workbench layout with standard 8px-or-smaller radii, fixed-height action rows, native icons where available, and no decorative cards. Required states are loading, empty, error with retry, upload progress, processing, exception, and completed. Long project names must wrap without overlapping state text.

The workflow is:

```text
项目列表 → 创建项目 → 查看采集计划 → 录音/选择照片
→ 上传并完成物料 → 启动自动处理 → 轮询状态 → 下载 PDF
```

- [ ] **Step 5: Run client tests and type checks**

Run:

```powershell
npm --prefix miniapp test
npm --prefix miniapp exec tsc -- --noEmit
```

Expected: all Vitest cases PASS and TypeScript reports no errors.

- [ ] **Step 6: Verify in WeChat DevTools**

Import `miniapp/project.config.json`, set the local API domain for development, create a project, upload both fixtures, start processing, wait for completion, and download the PDF. Capture desktop-width and narrow simulator screenshots and verify no overlapping controls or clipped text.

- [ ] **Step 7: Commit the mini-program**

```powershell
git add miniapp
git commit -m "feat: add staff mini-program vertical slice"
```

### Task 9: Add the container-backed end-to-end acceptance test

**Files:**
- Create: `test/integration/foundation_test.go`
- Create: `scripts/vertical-slice.ps1`
- Modify: `deploy/local/compose.yaml`
- Modify: `Makefile`
- Create: `README.md`

**Interfaces:**
- Consumes: the complete foundation slice.
- Produces: one command that verifies project creation through PDF download.

- [ ] **Step 1: Write the failing integration scenario**

```go
func TestFoundationVerticalSlice(t *testing.T) {
    api := newTestAPI(t)
    project := api.CreateProject(t, validProjectRequest())
    audio := api.UploadFixture(t, project.ID, "audio", "../../fixtures/interview.wav", "audio/wav")
    photo := api.UploadFixture(t, project.ID, "photo", "../../fixtures/family-photo.jpg", "image/jpeg")
    if audio.State != "uploaded" || photo.State != "uploaded" {
        t.Fatal("fixtures were not uploaded")
    }
    run := api.StartWorkflow(t, project.ID)
    completed := api.WaitForWorkflow(t, run.ID, 60*time.Second)
    if completed.ProjectState != "completed" {
        t.Fatalf("state = %s", completed.ProjectState)
    }
    pdf := api.DownloadLatestArtifact(t, project.ID)
    if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
        t.Fatal("artifact is not a PDF")
    }
}
```

- [ ] **Step 2: Verify the test fails before full stack startup**

Run: `go test ./test/integration -run TestFoundationVerticalSlice -v`

Expected: FAIL because the API base URL is unavailable.

- [ ] **Step 3: Implement the smoke script**

`scripts/vertical-slice.ps1` must:

1. Run `scripts/verify-prerequisites.ps1`.
2. Start Compose with `--build --wait`.
3. Apply migrations.
4. Wait for `/healthz`.
5. Run unit tests and the integration test.
6. Print the created project ID and artifact path.
7. Leave services running on success for manual mini-program verification.
8. On failure, print API and worker logs and return a non-zero exit code.

- [ ] **Step 4: Document local execution**

`README.md` must contain exact prerequisites, `.env` setup, Compose startup, migration, API/worker commands, mini-program import, smoke-test command, local endpoints, and a statement that AI outputs are deterministic fakes in this phase.

- [ ] **Step 5: Run the full acceptance gate**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
git diff --check
git status --short
```

Expected: unit, contract, integration, and mini-program API tests PASS; workflow reaches `completed`; PDF header is valid; `git diff --check` prints nothing.

- [ ] **Step 6: Commit the acceptance harness**

```powershell
git add test scripts/vertical-slice.ps1 deploy/local/compose.yaml Makefile README.md
git commit -m "test: verify foundation vertical slice"
```

## Final Verification

Run from `D:\Sangyu-record`:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-prerequisites.ps1
go test ./...
go vet ./...
npm --prefix skills/mock-memoir test
npm --prefix miniapp test
npm --prefix miniapp exec tsc -- --noEmit
powershell -ExecutionPolicy Bypass -File scripts/vertical-slice.ps1
git diff --check
git status --short --branch
```

Success requires:

- All Go, Node, and TypeScript tests pass.
- The Compose services are healthy.
- One project moves from `collecting` to `completed` without manual data editing.
- Duplicate workflow delivery does not duplicate node transitions.
- Uploaded source object keys remain unchanged.
- The generated manuscript retains evidence references.
- The downloaded artifact is a non-empty PDF.
- The working tree is clean after commits.

## Follow-On Plans

After this plan passes, create and review these separate plans in order:

1. `material-intelligence`: real speech-to-text, timestamps, diarization, OCR, and photo understanding.
2. `memory-and-retrieval`: memory units, timelines, people, conflicts, pgvector, and common-memory ingestion.
3. `memoir-skills`: Skill registry, positioning, chapter planning, writing, source maps, and cross-chapter consistency.
4. `quality-privacy-publishing`: authorization, sensitive-content policy, automated quality gates, and print-grade PDF.
5. `pilot-production`: staff authentication, project assignment, observability, deployment, backups, and pilot acceptance.
