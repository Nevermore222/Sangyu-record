# Sangyu Record Staff Miniapp System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete staff-only WeChat miniapp workflow from authenticated project management through multi-visit collection, staged Provider analysis, final memoir generation, PDF delivery, and local/production deployment guidance.

**Architecture:** Extend the existing Go API and PostgreSQL schema with staff sessions, consent, visits, asset associations, and visit analysis. Generalize the current durable workflow engine so both visit analysis and final book generation use the same Provider job, callback, polling, retry, and audit infrastructure. Keep the client as a native TypeScript WeChat miniapp with small service modules, persisted upload drafts, reusable visual components, and three stable tabs.

**Tech Stack:** Go 1.26, chi, pgx/PostgreSQL 17, Asynq/Redis, MinIO/S3, native WeChat miniapp TypeScript, Vitest, Docker Compose, Goose migrations, external Media/Knowledge/Agent Provider APIs.

## Global Constraints

- The product is staff-only; do not add elder, family, admin, publisher, payment, or logistics surfaces.
- Support multiple visits, multiple audio files, and multiple photos per elder.
- Support direct recording/camera plus WeChat file and album import; do not support video.
- Submitting a visit triggers transcription, photo understanding, material assessment, and follow-up planning only.
- Final book generation requires explicit staff confirmation and reuses the existing seven-node book workflow.
- Provider intelligence remains external; no model, knowledge base, or agent implementation belongs in this repository.
- Production uses WeChat login and HTTPS. `AUTH_MODE=dev` must be unavailable in production.
- Match the approved “人物志” visual direction: white base, charcoal text, burgundy action color, green success color, restrained editorial typography, compact operational layouts.
- Use TDD for every behavior change. Preserve existing Provider boundary and vertical-slice coverage.

---

### Task 1: Staff Authentication And Protected Routes

**Files:**
- Create: `migrations/00004_staff_auth.sql`
- Create: `internal/staff/model.go`
- Create: `internal/staff/repository.go`
- Create: `internal/staff/service.go`
- Create: `internal/staff/handler.go`
- Create: `internal/staff/middleware.go`
- Create: `internal/staff/service_test.go`
- Create: `internal/staff/handler_test.go`
- Create: `internal/staff/repository_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/httpapi/router.go`
- Modify: `internal/httpapi/router_test.go`
- Modify: `cmd/api/main.go`
- Modify: `.env.example`
- Modify: `deploy/local/compose.yaml`

**Interfaces:**
- Produces: `staff.Service.LoginWechat`, `staff.Service.LoginDev`, `staff.Service.Authenticate`, `staff.Middleware`, and authenticated `staff.FromContext(ctx)`.
- Consumes: WeChat `jscode2session` in production and an explicit local staff identity in dev mode.

- [ ] **Step 1: Add failing authentication and configuration tests**

```go
func TestDevLoginCreatesReusableSession(t *testing.T) {
    repo := newMemoryRepository()
    service := NewService(repo, Config{Mode: "dev", SessionTTL: 12 * time.Hour}, fixedClock)
    first, err := service.LoginDev(context.Background(), "Local Collector")
    if err != nil { t.Fatal(err) }
    staff, err := service.Authenticate(context.Background(), first.Token)
    if err != nil { t.Fatal(err) }
    if staff.DisplayName != "Local Collector" { t.Fatalf("staff = %#v", staff) }
}

func TestMiddlewareRejectsMissingBearerToken(t *testing.T) {
    handler := Middleware(fakeAuthenticator{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
    response := httptest.NewRecorder()
    handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/staff/me", nil))
    if response.Code != http.StatusUnauthorized { t.Fatalf("status = %d", response.Code) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/staff ./internal/config ./internal/httpapi -count=1`

Expected: build failure because the staff package, auth configuration, and middleware do not exist.

- [ ] **Step 3: Add migration and minimal domain implementation**

```sql
CREATE TYPE staff_state AS ENUM ('active', 'disabled');
CREATE TABLE staff (
    id uuid PRIMARY KEY,
    wechat_openid text NOT NULL UNIQUE,
    display_name text NOT NULL,
    team_name text NOT NULL DEFAULT '',
    state staff_state NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE staff_sessions (
    id uuid PRIMARY KEY,
    staff_id uuid NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    token_hash char(64) NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
```

```go
type Config struct {
    Mode          string
    WeChatAppID   string
    WeChatSecret  string
    AllowedOpenID map[string]struct{}
    SessionTTL    time.Duration
}

type LoginResult struct {
    Token string `json:"token"`
    Staff Staff  `json:"staff"`
}
```

Hash random 32-byte session tokens with SHA-256 before storage. Reject dev login unless `AUTH_MODE=dev`; reject process startup when production mode lacks WeChat credentials or session settings. Never log WeChat codes, session keys, or tokens.

- [ ] **Step 4: Register public auth routes and protect staff routes**

```go
router.Post("/v1/auth/wechat", authHandler.LoginWechat)
router.Post("/v1/auth/dev", authHandler.LoginDev)
router.Route("/v1/staff", func(router chi.Router) {
    router.Use(staffMiddleware.Handle)
    registerStaffRoutes(router)
})
```

Add `GET /v1/staff/me` and `POST /v1/staff/logout`. Return `401` for absent/expired sessions and `403` for disabled or non-allowlisted staff.

- [ ] **Step 5: Run migration and tests**

Run:

```powershell
$env:GOOSE_DRIVER='postgres'
$env:GOOSE_DBSTRING='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
$env:TEST_DATABASE_URL=$env:GOOSE_DBSTRING
go test ./internal/staff ./internal/config ./internal/httpapi ./cmd/api -count=1
```

Expected: migration version 4 and all focused tests pass.

- [ ] **Step 6: Commit**

```powershell
git add migrations/00004_staff_auth.sql internal/staff internal/config internal/httpapi cmd/api/main.go .env.example deploy/local/compose.yaml
git commit -m "feat: authenticate staff sessions"
```

---

### Task 2: Server-Backed Project Lists, Dashboard, And Consent

**Files:**
- Create: `migrations/00005_project_ownership_and_consent.sql`
- Modify: `internal/projects/model.go`
- Modify: `internal/projects/repository.go`
- Modify: `internal/projects/service.go`
- Modify: `internal/projects/handler.go`
- Modify: `internal/projects/handler_test.go`
- Modify: `internal/projects/repository_test.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: `staff.FromContext(ctx).ID`.
- Produces: `projects.Page`, `projects.Dashboard`, `projects.Consent`, list/dashboard APIs, and `projects.Repository.HasConsent` for visits/finalization.

- [ ] **Step 1: Write failing repository and handler tests**

```go
func TestListReturnsOnlyOwnedProjectsWithCursor(t *testing.T) {
    page, err := repo.List(ctx, ListInput{OwnerStaffID: ownerID, Limit: 2})
    if err != nil { t.Fatal(err) }
    if len(page.Items) != 2 || page.NextCursor == "" { t.Fatalf("page = %#v", page) }
}

func TestConsentIsIdempotentPerConfirmation(t *testing.T) {
    first, err := service.ConfirmConsent(ctx, projectID, staffID, ConfirmConsentInput{ConfirmedBy: "elder"})
    if err != nil { t.Fatal(err) }
    second, err := service.ConfirmConsent(ctx, projectID, staffID, ConfirmConsentInput{ConfirmedBy: "elder"})
    if err != nil { t.Fatal(err) }
    if first.ID != second.ID { t.Fatal("duplicate consent record") }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/projects -count=1`

Expected: build failure because list, dashboard, ownership, and consent APIs are absent.

- [ ] **Step 3: Add ownership and consent schema**

```sql
ALTER TABLE projects ADD COLUMN owner_staff_id uuid REFERENCES staff(id);
CREATE TABLE consents (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    confirmed_by text NOT NULL CHECK (confirmed_by IN ('elder','guardian')),
    confirmation_method text NOT NULL CHECK (confirmation_method = 'onsite'),
    staff_id uuid NOT NULL REFERENCES staff(id),
    confirmed_at timestamptz NOT NULL,
    UNIQUE (project_id, confirmed_by, confirmation_method)
);
CREATE INDEX projects_owner_updated_idx ON projects(owner_staff_id, updated_at DESC, id DESC);
```

Existing projects remain readable in dev mode. New production projects always persist `owner_staff_id` from the authenticated context.

- [ ] **Step 4: Implement cursor pagination and dashboard aggregation**

```go
type ListInput struct {
    OwnerStaffID uuid.UUID
    Query        string
    State        State
    Cursor       string
    Limit        int
}

type Page struct {
    Items      []ProjectSummary `json:"items"`
    NextCursor string           `json:"next_cursor,omitempty"`
}
```

Encode the cursor from `(updated_at,id)`. Clamp limit to `1..50`. Search only display name, birthplace, residence, and occupation using parameterized `ILIKE` expressions. Dashboard returns counts plus at most five recent actionable projects.

- [ ] **Step 5: Register routes and verify responses**

Add:

```text
GET  /v1/staff/dashboard
GET  /v1/staff/projects?query=&state=&cursor=&limit=
POST /v1/staff/projects/{projectID}/consents
```

Return consent summary and visit/workflow summaries from project detail without exposing Provider payloads.

- [ ] **Step 6: Run focused and full project tests**

Run:

```powershell
go test ./internal/projects -count=1
go test ./... -count=1
```

Expected: all tests pass and existing project creation remains compatible.

- [ ] **Step 7: Commit**

```powershell
git add migrations/00005_project_ownership_and_consent.sql internal/projects cmd/api/main.go
git commit -m "feat: list staff projects and consent"
```

---

### Task 3: Visits And Visit-Scoped Assets

**Files:**
- Create: `migrations/00006_visits_and_assets.sql`
- Create: `internal/visits/model.go`
- Create: `internal/visits/repository.go`
- Create: `internal/visits/service.go`
- Create: `internal/visits/handler.go`
- Create: `internal/visits/service_test.go`
- Create: `internal/visits/handler_test.go`
- Create: `internal/visits/repository_test.go`
- Modify: `internal/assets/model.go`
- Modify: `internal/assets/repository.go`
- Modify: `internal/assets/service.go`
- Modify: `internal/assets/handler.go`
- Modify: `internal/assets/service_test.go`
- Modify: `internal/assets/repository_test.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Produces: `visits.Service.Create/Update/List/Get`, visit-scoped asset APIs, `assets.Service.RenewUpload`, and pending-asset deletion.
- Consumes: authenticated staff ID and `projects.Repository.HasConsent`.

- [ ] **Step 1: Write failing visit state and upload renewal tests**

```go
func TestCreateVisitRequiresConsent(t *testing.T) {
    _, err := service.Create(ctx, CreateInput{ProjectID: projectID, StaffID: staffID})
    if !errors.Is(err, ErrConsentRequired) { t.Fatalf("err = %v", err) }
}

func TestRenewUploadKeepsAssetIdentityAndObjectKey(t *testing.T) {
    renewed, err := service.RenewUpload(ctx, pendingAsset.ID)
    if err != nil { t.Fatal(err) }
    if renewed.AssetID != pendingAsset.ID { t.Fatal("renewal replaced asset") }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/visits ./internal/assets -count=1`

Expected: build failure because visits and renewal do not exist.

- [ ] **Step 3: Add visit and asset-association migration**

```sql
CREATE TYPE visit_state AS ENUM ('draft','submitted','analyzing','completed','failed');
CREATE TYPE asset_source AS ENUM ('direct','wechat_file','album','camera');
CREATE TABLE visits (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sequence integer NOT NULL,
    staff_id uuid NOT NULL REFERENCES staff(id),
    visited_at timestamptz NOT NULL,
    location text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    state visit_state NOT NULL DEFAULT 'draft',
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(project_id, sequence)
);
CREATE TABLE visit_plan_items (
    visit_id uuid NOT NULL REFERENCES visits(id) ON DELETE CASCADE,
    plan_item_id uuid NOT NULL REFERENCES collection_plan_items(id) ON DELETE CASCADE,
    PRIMARY KEY(visit_id, plan_item_id)
);
ALTER TABLE assets ADD COLUMN visit_id uuid REFERENCES visits(id) ON DELETE CASCADE;
ALTER TABLE assets ADD COLUMN source asset_source;
ALTER TABLE assets ADD COLUMN display_name text NOT NULL DEFAULT '';
CREATE TABLE asset_plan_items (
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    plan_item_id uuid NOT NULL REFERENCES collection_plan_items(id) ON DELETE CASCADE,
    PRIMARY KEY(asset_id, plan_item_id)
);
```

- [ ] **Step 4: Implement visit draft lifecycle**

Use one transaction with `SELECT ... FOR UPDATE` on the project to allocate the next sequence. Only `draft` visits can update location, notes, or selected plan items. Enforce project ownership in every service method.

```go
type CreateInput struct {
    ProjectID   uuid.UUID
    StaffID     uuid.UUID
    VisitedAt   time.Time
    Location    string
    PlanItemIDs []uuid.UUID
}
```

- [ ] **Step 5: Extend assets without breaking old callers**

Add optional `VisitID`, `Source`, `DisplayName`, and `PlanItemIDs` to `InitiateInput`. New authenticated miniapp requests require a draft visit; existing integration fixtures may omit visit ID. `RenewUpload` only accepts `pending_upload`. `DeletePending` deletes the database row and best-effort removes a partially uploaded object.

- [ ] **Step 6: Register and test routes**

```text
POST   /v1/staff/projects/{projectID}/visits
GET    /v1/staff/projects/{projectID}/visits
GET    /v1/staff/visits/{visitID}
PATCH  /v1/staff/visits/{visitID}
GET    /v1/staff/visits/{visitID}/assets
POST   /v1/staff/assets/{assetID}:renew-upload
DELETE /v1/staff/assets/{assetID}
```

Run: `go test ./internal/visits ./internal/assets ./... -count=1`

Expected: all tests pass.

- [ ] **Step 7: Commit**

```powershell
git add migrations/00006_visits_and_assets.sql internal/visits internal/assets cmd/api/main.go
git commit -m "feat: manage collection visits and assets"
```

---

### Task 4: Generalize Durable Workflows For Visit Analysis

**Files:**
- Create: `migrations/00007_workflow_kinds.sql`
- Modify: `internal/workflow/model.go`
- Modify: `internal/workflow/repository.go`
- Modify: `internal/workflow/repository_test.go`
- Modify: `internal/workflow/service.go`
- Modify: `internal/workflow/service_test.go`
- Modify: `internal/workflow/processors.go`
- Modify: `internal/workflow/processors_test.go`
- Modify: `internal/workflow/worker_test.go`
- Modify: `test/integration/foundation_test.go`

**Interfaces:**
- Produces: `workflow.RunKindBook`, `workflow.RunKindVisitAnalysis`, position-based node progression, and `CreateRunInput`.
- Preserves: existing `Service.Start(projectID)` behavior and seven book nodes.

- [ ] **Step 1: Write failing dynamic-sequence tests**

```go
func TestVisitAnalysisRunUsesItsOwnSequence(t *testing.T) {
    run, err := repo.CreateRun(ctx, CreateRunInput{
        ProjectID: projectID,
        VisitID: visitID,
        Kind: RunKindVisitAnalysis,
        Nodes: VisitAnalysisSequence,
    })
    if err != nil { t.Fatal(err) }
    if len(run.Nodes) != len(VisitAnalysisSequence) { t.Fatalf("nodes = %#v", run.Nodes) }
}

func TestBookRunStillUsesSevenNodes(t *testing.T) {
    run, err := service.Start(ctx, projectID)
    if err != nil { t.Fatal(err) }
    if len(run.Nodes) != 7 { t.Fatalf("nodes = %d", len(run.Nodes)) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/workflow ./test/integration -count=1`

Expected: failure because workflow kind, visit ID, and node position are absent.

- [ ] **Step 3: Add workflow kind and ordered-node schema**

```sql
CREATE TYPE workflow_kind AS ENUM ('book','visit_analysis');
ALTER TABLE workflow_runs ADD COLUMN kind workflow_kind NOT NULL DEFAULT 'book';
ALTER TABLE workflow_runs ADD COLUMN visit_id uuid REFERENCES visits(id) ON DELETE CASCADE;
ALTER TABLE workflow_nodes ADD COLUMN position integer;
UPDATE workflow_nodes SET position = CASE node_name
  WHEN 'transcribe' THEN 0 WHEN 'understand_photo' THEN 1 WHEN 'build_memory' THEN 2
  WHEN 'retrieve_shared_memory' THEN 3 WHEN 'plan_book' THEN 4
  WHEN 'write_book' THEN 5 WHEN 'render_pdf' THEN 6 END;
ALTER TABLE workflow_nodes ALTER COLUMN position SET NOT NULL;
CREATE UNIQUE INDEX workflow_nodes_position_idx ON workflow_nodes(run_id, position);
```

- [ ] **Step 4: Replace global next-node lookup with persisted position**

```go
type CreateRunInput struct {
    ProjectID uuid.UUID
    VisitID   uuid.UUID
    Kind      RunKind
    Nodes     []NodeName
}
```

`SucceedNode` selects the next queued node using `position > current.position ORDER BY position LIMIT 1`. On final book completion update project state; on visit-analysis completion leave project state unchanged for the visit completion processor.

- [ ] **Step 5: Run regression tests**

Run:

```powershell
$env:TEST_DATABASE_URL='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go test ./internal/workflow ./internal/providerjobs ./test/integration -count=1
```

Expected: old seven-node behavior and new dynamic sequence tests pass.

- [ ] **Step 6: Commit**

```powershell
git add migrations/00007_workflow_kinds.sql internal/workflow test/integration/foundation_test.go
git commit -m "refactor: support typed workflow sequences"
```

---

### Task 5: Visit Analysis Providers, Persistence, And Recovery

**Files:**
- Create: `migrations/00008_visit_analysis.sql`
- Create: `internal/visitanalysis/model.go`
- Create: `internal/visitanalysis/repository.go`
- Create: `internal/visitanalysis/service.go`
- Create: `internal/visitanalysis/handler.go`
- Create: `internal/visitanalysis/processor.go`
- Create: `internal/visitanalysis/service_test.go`
- Create: `internal/visitanalysis/repository_test.go`
- Create: `internal/visitanalysis/processor_test.go`
- Modify: `internal/providers/normalize.go`
- Modify: `internal/providers/normalize_test.go`
- Modify: `internal/workflow/model.go`
- Modify: `internal/workflow/processors.go`
- Modify: `providers/mock/server.mjs`
- Modify: `providers/mock/server.test.mjs`
- Modify: `cmd/api/main.go`
- Modify: `cmd/worker/main.go`

**Interfaces:**
- Produces: `visitanalysis.Service.Submit/Retry/Get`, material-assessment and follow-up normalizers, and a terminal persistence processor.
- Consumes: generalized workflow engine, visit-scoped asset URL reader, and existing Provider job service.

- [ ] **Step 1: Write failing Provider normalization tests**

```go
func TestNormalizeMaterialAssessment(t *testing.T) {
    raw := json.RawMessage(`{"complete":false,"covered_items":[{"plan_item_id":"p1","evidence_refs":["audio#1-5"]}],"gaps":[{"plan_item_id":"p2","reason":"missing detail"}]}`)
    normalized, err := Normalize(TaskMaterialAssessment, raw)
    if err != nil || !json.Valid(normalized) { t.Fatalf("normalized=%s err=%v", normalized, err) }
}

func TestNormalizeFollowupPlanRejectsEmptyQuestions(t *testing.T) {
    _, err := Normalize(TaskFollowupPlan, json.RawMessage(`{"questions":[]}`))
    if !errors.Is(err, ErrInvalidOutput) { t.Fatalf("err = %v", err) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/providers ./internal/visitanalysis -count=1`

Expected: unsupported-task failure and missing visitanalysis package.

- [ ] **Step 3: Add analysis schema and models**

```sql
CREATE TABLE visit_analyses (
    id uuid PRIMARY KEY,
    visit_id uuid NOT NULL UNIQUE REFERENCES visits(id) ON DELETE CASCADE,
    workflow_run_id uuid NOT NULL UNIQUE REFERENCES workflow_runs(id) ON DELETE CASCADE,
    summary text NOT NULL,
    covered_items jsonb NOT NULL,
    gaps jsonb NOT NULL,
    followup_questions jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE collection_plan_items ADD COLUMN gap_reason text NOT NULL DEFAULT '';
```

Define visit nodes in order: `visit_transcribe`, `visit_understand_photo`, `visit_assess_material`, `visit_plan_followup`, `visit_persist_analysis`.

- [ ] **Step 4: Implement visit-scoped Provider inputs**

Media nodes receive only URLs belonging to the visit. Agent nodes receive normalized upstream node outputs, selected plan items, and project context as structured JSON. They receive no object URLs.

```go
var VisitAnalysisSequence = []workflow.NodeName{
    workflow.NodeVisitTranscribe,
    workflow.NodeVisitUnderstandPhoto,
    workflow.NodeVisitAssessMaterial,
    workflow.NodeVisitPlanFollowup,
    workflow.NodeVisitPersistAnalysis,
}
```

- [ ] **Step 5: Persist analysis and plan updates atomically**

The final local processor reads upstream node outputs, writes one `visit_analyses` row, updates covered collection items to `collected`, updates gaps to `insufficient` with `gap_reason`, and marks the visit `completed` in one transaction. Reprocessing the same run returns the existing analysis.

- [ ] **Step 6: Add submit/retry/query routes**

```text
POST /v1/staff/visits/{visitID}:submit
POST /v1/staff/visits/{visitID}:retry
GET  /v1/staff/visits/{visitID}/analysis
```

Submission requires a draft visit with at least one uploaded audio or photo. Retry creates no duplicate Provider submissions because workflow node and Provider idempotency keys remain stable.

- [ ] **Step 7: Run Provider, workflow, and analysis tests**

Run:

```powershell
go test ./internal/providers ./internal/visitanalysis ./internal/workflow ./internal/providerjobs -count=1
npm --prefix providers/mock test
```

Expected: all tests pass, including malformed assessment output and retry recovery.

- [ ] **Step 8: Commit**

```powershell
git add migrations/00008_visit_analysis.sql internal/visitanalysis internal/providers internal/workflow providers/mock cmd/api/main.go cmd/worker/main.go
git commit -m "feat: analyze collection visits"
```

---

### Task 6: Explicit Finalization And Book Workflow Guardrails

**Files:**
- Modify: `internal/workflow/service.go`
- Modify: `internal/workflow/handler.go`
- Modify: `internal/workflow/service_test.go`
- Modify: `internal/workflow/handler_test.go`
- Modify: `internal/workflow/repository.go`
- Modify: `internal/projects/model.go`

**Interfaces:**
- Produces: `workflow.FinalizeInput{ConfirmMaterialsReady bool}` and idempotent `POST /projects/{projectID}:finalize`.
- Consumes: consent, visit, asset, and latest-book-run readiness queries.

- [ ] **Step 1: Write failing finalization guard tests**

```go
func TestFinalizeRequiresExplicitConfirmation(t *testing.T) {
    _, err := service.Finalize(ctx, projectID, FinalizeInput{})
    if !errors.Is(err, ErrConfirmationRequired) { t.Fatalf("err = %v", err) }
}

func TestFinalizeRejectsDraftVisit(t *testing.T) {
    _, err := service.Finalize(ctx, projectID, FinalizeInput{ConfirmMaterialsReady: true})
    if !errors.Is(err, ErrDraftVisitExists) { t.Fatalf("err = %v", err) }
}

func TestFinalizeReturnsExistingActiveBookRun(t *testing.T) {
    first, _ := service.Finalize(ctx, projectID, confirmed)
    second, _ := service.Finalize(ctx, projectID, confirmed)
    if first.ID != second.ID { t.Fatal("duplicate book run") }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/workflow -run Finalize -count=1`

Expected: build failure because finalization guards do not exist.

- [ ] **Step 3: Implement readiness checks and route**

Check explicit confirmation, valid consent, no draft visit, uploaded audio and photo, and no active book run. Keep the existing `/workflow:start` route for compatibility, but make the miniapp call `POST /projects/{projectID}:finalize`.

- [ ] **Step 4: Run workflow and integration regression tests**

Run: `go test ./internal/workflow ./test/integration -count=1`

Expected: finalization tests and existing vertical slice pass.

- [ ] **Step 5: Commit**

```powershell
git add internal/workflow internal/projects
git commit -m "feat: guard final memoir generation"
```

---

### Task 7: Miniapp Session, API, State Presentation, And Visual Foundation

**Files:**
- Create: `miniapp/services/session.ts`
- Create: `miniapp/services/session.test.ts`
- Create: `miniapp/services/upload-queue.ts`
- Create: `miniapp/services/upload-queue.test.ts`
- Create: `miniapp/services/drafts.ts`
- Create: `miniapp/services/drafts.test.ts`
- Create: `miniapp/domain/presenters.ts`
- Create: `miniapp/domain/presenters.test.ts`
- Create: `miniapp/components/status-tag/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/empty-state/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/action-bar/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/login/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/services/api.ts`
- Modify: `miniapp/services/api.test.ts`
- Modify: `miniapp/services/client.ts`
- Modify: `miniapp/app.ts`
- Modify: `miniapp/app.json`
- Modify: `miniapp/app.wxss`
- Modify: `miniapp/env.ts`

**Interfaces:**
- Produces: authenticated API client, `UploadQueue`, draft storage, status presenters, and global visual tokens.
- Consumes: Task 1-6 API contracts.

- [ ] **Step 1: Write failing client-state tests**

```ts
it('refreshes an expired session once and replays the request', async () => {
  const api = createAPI({ request, session })
  const project = await api.getProject('project-1')
  expect(project.id).toBe('project-1')
  expect(session.refreshCalls).toBe(1)
})

it('keeps failed queue items while completed items are removed', async () => {
  const queue = createUploadQueue(storage, uploader)
  await queue.resume()
  expect(queue.snapshot().map(item => item.state)).toEqual(['failed'])
})
```

- [ ] **Step 2: Run tests and verify RED**

Run: `npm --prefix miniapp test`

Expected: missing session, queue, drafts, and presenter modules.

- [ ] **Step 3: Implement session and API contracts**

Use `wx.login` for production and `/v1/auth/dev` only when `env.AUTH_MODE === 'dev'`. Store only the opaque token and staff summary. Add `Authorization` to protected calls. Replay a failed `401` request once after relogin; never loop.

- [ ] **Step 4: Implement persisted upload queue**

```ts
export interface UploadQueueItem {
  localID: string
  visitID: string
  assetID?: string
  filePath: string
  kind: 'audio' | 'photo'
  source: 'direct' | 'wechat_file' | 'album' | 'camera'
  state: 'local' | 'uploading' | 'completed' | 'failed'
  progress: number
  error?: string
}
```

Persist metadata with `wx.setStorageSync`; persist captured files with `wx.saveFile`; delete a local file only after the asset completion API succeeds. Renew expired tickets using the exact asset ID.

- [ ] **Step 5: Apply approved visual foundation**

Set CSS variables to the approved palette, use Songti for display text and PingFang for body text, keep card radius at 6px, provide stable 88rpx controls, safe-area action bars, visible focus/pressed/disabled states, and no decorative gradients. Fix all current mojibake text in `miniapp/`.

- [ ] **Step 6: Run unit tests and typecheck**

Run:

```powershell
npm --prefix miniapp test
npm --prefix miniapp run typecheck
```

Expected: all miniapp tests and TypeScript checks pass.

- [ ] **Step 7: Commit**

```powershell
git add miniapp
git commit -m "feat: establish miniapp application shell"
```

---

### Task 8: Workbench, Project List, Creation, And Project Detail UI

**Files:**
- Create: `miniapp/pages/workbench/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/profile/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/project-row/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/progress-steps/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/pages/projects/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/pages/create/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/pages/project/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/app.json`
- Modify: `miniapp/services/api.test.ts`

**Interfaces:**
- Consumes: dashboard, paginated projects, consent, visits, workflow, and artifact summaries.
- Produces: the three-tab navigation and server-backed archive workflow.

- [ ] **Step 1: Add failing presenter/API tests for page data**

```ts
it('maps actionable dashboard projects in priority order', () => {
  const rows = presentDashboard(input)
  expect(rows.map(row => row.action)).toEqual(['补充采集', '查看报告', '继续处理'])
})

it('uses the server project list instead of local project IDs', async () => {
  await api.listProjects({ limit: 20 })
  expect(request).toHaveBeenCalledWith(expect.objectContaining({ url: expect.stringContaining('/v1/staff/projects?') }))
})
```

- [ ] **Step 2: Run tests and verify RED**

Run: `npm --prefix miniapp test`

Expected: missing dashboard presenter/list API.

- [ ] **Step 3: Build three-tab shell and workbench**

Use native tabBar for 工作台、档案、我的. Workbench shows only actionable counts, priority rows, recent archives, and one primary “建立老人档案” action. It must render loading, empty, error, and populated states without layout shifts.

- [ ] **Step 4: Build server-backed project list and two-step creation**

Add debounced search, state filters, pull-to-refresh, and cursor pagination. Creation validates each step locally, submits once, records consent, then redirects to project detail. Do not cache project IDs as source of truth.

- [ ] **Step 5: Build project detail**

Show plan progress, task gap reasons, visit history, latest analysis, workflow status, and artifact entry. Expose “开始新走访” while collecting and “确认资料齐备并成书” only when readiness conditions are met.

- [ ] **Step 6: Test and typecheck**

Run: `npm --prefix miniapp test; npm --prefix miniapp run typecheck`

Expected: all tests pass.

- [ ] **Step 7: Commit**

```powershell
git add miniapp/pages miniapp/components miniapp/app.json miniapp/services
git commit -m "feat: build staff archive workspace"
```

---

### Task 9: Visit Preparation, Capture, Upload, Analysis, And PDF UI

**Files:**
- Create: `miniapp/pages/visit-prepare/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/visit-capture/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/visit-report/index.{json,ts,wxml,wxss}`
- Create: `miniapp/pages/workflow/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/asset-row/index.{json,ts,wxml,wxss}`
- Create: `miniapp/components/photo-grid/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/pages/result/index.{json,ts,wxml,wxss}`
- Remove after replacement: `miniapp/pages/upload/index.{json,ts,wxml,wxss}`
- Modify: `miniapp/app.json`
- Modify: `miniapp/services/upload-queue.test.ts`

**Interfaces:**
- Consumes: visits, visit assets, submit/retry analysis, finalization, workflow, and artifact APIs.
- Produces: the complete field collection and delivery journey.

- [ ] **Step 1: Add failing upload/capture state tests**

```ts
it('restores a saved visit draft after page reload', () => {
  const draft = drafts.load('visit-1')
  expect(draft.items).toHaveLength(3)
})

it('blocks visit submission while any upload is incomplete', () => {
  expect(canSubmitVisit([{ state: 'completed' }, { state: 'failed' }])).toBe(false)
})
```

- [ ] **Step 2: Run tests and verify RED**

Run: `npm --prefix miniapp test`

Expected: missing capture state helpers.

- [ ] **Step 3: Build visit preparation and capture**

Preparation lets staff select plan items and set date/location. Capture supports recorder start/stop, `wx.chooseMessageFile` for imported audio, `wx.chooseMedia` for camera/album photos, per-item rename/association/removal, notes, and upload progress. Keep button and grid dimensions stable while recording or uploading.

- [ ] **Step 4: Build safe exit and draft recovery**

Use `wx.enableAlertBeforeUnload` while local or failed items exist. Resume the queue on `onShow`. If a persisted file is missing, mark only that item failed with “本地文件已失效，请重新选择”.

- [ ] **Step 5: Build visit report, workflow, and result**

Report displays normalized summary, covered tasks, gaps, and follow-up questions. Workflow maps technical nodes to Chinese business labels. Result formats file size, opens PDF with `wx.openDocument`, and keeps the native share menu enabled.

- [ ] **Step 6: Remove obsolete upload page and run checks**

Run:

```powershell
npm --prefix miniapp test
npm --prefix miniapp run typecheck
```

Expected: all tests pass and no page references `pages/upload/index`.

- [ ] **Step 7: Commit**

```powershell
git add miniapp
git commit -m "feat: complete field collection journey"
```

---

### Task 10: End-To-End Acceptance, Local Launcher, And Production Deployment

**Files:**
- Modify: `test/integration/foundation_test.go`
- Create: `test/integration/staff_miniapp_test.go`
- Modify: `scripts/vertical-slice.ps1`
- Create: `scripts/miniapp-local.ps1`
- Create: `deploy/production/compose.yaml`
- Create: `deploy/production/Caddyfile`
- Create: `deploy/production/.env.production.example`
- Create: `docs/deployment-miniapp.md`
- Modify: `README.md`

**Interfaces:**
- Produces: one-command local startup, deployment-ready production templates, and an integration proof covering the entire staff workflow.
- Consumes: all previous tasks.

- [ ] **Step 1: Write the failing staff vertical-slice test**

```go
func TestStaffMiniappVerticalSlice(t *testing.T) {
    token := api.devLogin(t)
    project := api.createProject(t, token)
    api.confirmConsent(t, token, project.ID)
    first := api.createVisit(t, token, project.ID)
    api.uploadVisitFixtures(t, token, first.ID, 2, 3)
    api.submitAndWaitForVisitAnalysis(t, token, first.ID)
    second := api.createVisit(t, token, project.ID)
    api.uploadVisitFixtures(t, token, second.ID, 1, 1)
    api.submitAndWaitForVisitAnalysis(t, token, second.ID)
    api.finalizeAndWait(t, token, project.ID)
    pdf := api.downloadLatestPDF(t, token, project.ID)
    if !bytes.HasPrefix(pdf, []byte("%PDF-")) { t.Fatal("artifact is not PDF") }
}
```

- [ ] **Step 2: Run integration test and verify RED**

Run:

```powershell
$env:TEST_API_URL='http://localhost:8080'
$env:TEST_DATABASE_URL='postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
go test ./test/integration -run TestStaffMiniappVerticalSlice -v -count=1
```

Expected: failure until all new routes and visit workflow are wired into the running Compose stack.

- [ ] **Step 3: Complete local launcher**

`scripts/miniapp-local.ps1` must:

1. verify Docker, Go, Node/npm, and optional WeChat DevTools CLI;
2. run Compose with `--build --wait`;
3. apply Goose migrations;
4. run Go tests/vet, Mock Provider tests, miniapp tests/typecheck;
5. run `TestStaffMiniappVerticalSlice`;
6. print API, MinIO, mock Provider, project import path, and dev login details;
7. leave services running.

- [ ] **Step 4: Add production templates**

Use Caddy for HTTPS API and callback routing. Do not publish PostgreSQL, Redis, MinIO API, Chromium, or Provider mock ports. Do not include real secrets. Add health checks and persistent volumes. Document request/upload/download legal domains and WeChat experience-version upload steps.

- [ ] **Step 5: Run full acceptance**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/miniapp-local.ps1
git diff --check
git status --short
```

Expected:

- all Go tests and `go vet` pass;
- all Mock Provider and miniapp tests pass;
- typecheck passes;
- two visits produce analysis and a final PDF;
- all Docker services are healthy;
- API health returns `200`;
- services remain running for WeChat DevTools.

- [ ] **Step 6: Verify the real miniapp in WeChat DevTools**

Import `D:\Sangyu-record\miniapp\project.config.json`, build npm, disable domain validation only for local development, then verify login, project creation, consent, direct recording, photo selection, imported audio, draft restoration, visit report, final workflow, and PDF opening. Capture desktop and narrow-device screenshots and correct any overlap, clipping, or unreadable status text before completion.

- [ ] **Step 7: Commit**

```powershell
git add test/integration scripts deploy/production docs/deployment-miniapp.md README.md
git commit -m "test: deliver staff miniapp deployment"
```

---

## Plan Self-Review

- Every approved spec requirement maps to at least one task.
- Staff authentication precedes ownership and protected business routes.
- Visits and visit-scoped assets exist before visit-analysis workflows.
- Workflow generalization preserves the existing seven-node book flow before adding visit nodes.
- The miniapp API foundation lands before page implementation.
- Local and production deployment are separate; production never enables dev auth or exposes internal service ports.
- No task introduces video, elder/family UI, admin UI, offline sync, online layout editing, payment, or publisher ordering.
