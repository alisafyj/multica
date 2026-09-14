package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const taskRunEvidenceTestClaimGeneration = int64(100_000_000)

func validTaskRunEvidenceRequest() TaskRunEvidenceRequest {
	zero := int64(0)
	input := int64(120)
	cacheRead := int64(20)
	cacheWrite := int64(5)
	output := int64(10)
	cost := int64(19990000000)
	model := "glm-5.3"
	effort := "high"
	version := "0.153.4"
	hash := "sha256:" + strings.Repeat("a", 64)
	return TaskRunEvidenceRequest{
		SchemaVersion:   taskRunEvidenceSchemaVersion,
		Attempt:         2,
		ClaimGeneration: taskRunEvidenceTestClaimGeneration,
		Revision:        1,
		Timings: TaskRunEvidenceTimings{
			Queue:        TaskRunEvidenceTiming{Known: true, DurationMS: &zero},
			Preparation:  TaskRunEvidenceTiming{Known: true, DurationMS: &zero},
			FirstTool:    TaskRunEvidenceTiming{Known: false},
			Execution:    TaskRunEvidenceTiming{Known: true, DurationMS: &zero},
			Finalization: TaskRunEvidenceTiming{Known: true, DurationMS: &zero},
		},
		Requested:       TaskRunEvidenceModelConfig{Model: &model, Effort: &effort},
		ClientEffective: TaskRunEvidenceModelConfig{Model: &model, Effort: &effort},
		ProviderReported: TaskRunEvidenceProviderModel{
			Model:  &model,
			Source: "model_usage",
		},
		Runtime: TaskRunEvidenceRuntime{
			Version:       &version,
			ContentSHA256: &hash,
		},
		Usage: TaskRunEvidenceUsage{
			InputUncachedTokens:   &input,
			InputCacheReadTokens:  &cacheRead,
			InputCacheWriteTokens: &cacheWrite,
			OutputTokens:          &output,
			Complete:              true,
			Source:                "provider_event",
		},
		ProviderCost: TaskRunEvidenceProviderCost{
			AmountUSDTicks: &cost,
			Complete:       false,
			Authority:      "provider_reported",
			Basis:          "unknown",
			Source:         "model_usage",
		},
	}
}

func completeTaskRunEvidenceModelUsage() *TaskRunEvidenceModelUsageInventory {
	max := int64(math.MaxInt64)
	missingCost := TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"}
	usage := TaskRunEvidenceUsage{
		InputUncachedTokens: &max, InputCacheReadTokens: &max,
		InputCacheWriteTokens: &max, OutputTokens: &max,
		Complete: true, Source: "model_usage",
	}
	return &TaskRunEvidenceModelUsageInventory{
		Entries: []TaskRunEvidenceModelUsage{
			{Model: "a-model", Usage: usage, ProviderCost: missingCost},
			{Model: "z-model", Usage: usage, ProviderCost: missingCost},
		},
		Complete: true, Source: "model_usage",
	}
}

func TestValidateTaskRunEvidenceModelUsageInventory(t *testing.T) {
	valid := validTaskRunEvidenceRequest()
	valid.ModelUsage = completeTaskRunEvidenceModelUsage()
	if err := validateTaskRunEvidenceRequest(valid); err != nil {
		t.Fatalf("complete inventory with unknown provider costs rejected: %v", err)
	}

	missing := validTaskRunEvidenceRequest()
	missing.ModelUsage = &TaskRunEvidenceModelUsageInventory{Entries: []TaskRunEvidenceModelUsage{}, Source: "missing"}
	if err := validateTaskRunEvidenceRequest(missing); err != nil {
		t.Fatalf("canonical missing inventory rejected: %v", err)
	}

	unknownUsage := validTaskRunEvidenceRequest()
	unknownUsage.ModelUsage = &TaskRunEvidenceModelUsageInventory{
		Entries: []TaskRunEvidenceModelUsage{{
			Model:        "a-model",
			Usage:        TaskRunEvidenceUsage{Source: "missing"},
			ProviderCost: TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"},
		}},
		Complete: false,
		Source:   "model_usage",
	}
	if err := validateTaskRunEvidenceRequest(unknownUsage); err != nil {
		t.Fatalf("observed model with unknown normalized buckets rejected: %v", err)
	}

	tests := map[string]func(*TaskRunEvidenceModelUsageInventory){
		"null entries": func(value *TaskRunEvidenceModelUsageInventory) { value.Entries = nil },
		"too many entries": func(value *TaskRunEvidenceModelUsageInventory) {
			value.Entries = append(value.Entries, make([]TaskRunEvidenceModelUsage, 15)...)
		},
		"unsorted entries": func(value *TaskRunEvidenceModelUsageInventory) {
			value.Entries[0], value.Entries[1] = value.Entries[1], value.Entries[0]
		},
		"duplicate entries":                        func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[1].Model = value.Entries[0].Model },
		"empty model":                              func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Model = "" },
		"oversized model":                          func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Model = strings.Repeat("a", 256) },
		"control in model":                         func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Model = "a\nmodel" },
		"invalid utf8 model":                       func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Model = string([]byte{0xff}) },
		"invalid nested usage":                     func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Usage.OutputTokens = nil },
		"invalid nested cost":                      func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].ProviderCost.Complete = true },
		"complete truncated inventory":             func(value *TaskRunEvidenceModelUsageInventory) { value.Truncated = true },
		"complete inventory with incomplete usage": func(value *TaskRunEvidenceModelUsageInventory) { value.Entries[0].Usage.Complete = false },
		"missing source with entries":              func(value *TaskRunEvidenceModelUsageInventory) { value.Source = "missing" },
		"unknown source":                           func(value *TaskRunEvidenceModelUsageInventory) { value.Source = "provider_summary" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			req := validTaskRunEvidenceRequest()
			req.ModelUsage = completeTaskRunEvidenceModelUsage()
			mutate(req.ModelUsage)
			if err := validateTaskRunEvidenceRequest(req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDecodeTaskRunEvidenceModelUsageRejectsUnknownFields(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.ModelUsage = completeTaskRunEvidenceModelUsage()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte(`"model":"a-model"`), []byte(`"model":"a-model","unknown":true`), 1)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()
	if _, ok := decodeTaskRunEvidenceRequest(w, r); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("unknown nested field accepted: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDecodeTaskRunEvidenceModelUsageNegotiatedCapFitsBodyLimit(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	models := make([]string, 16)
	escapedAlphabet := []byte{'<', '>', '&'}
	for i := range models {
		suffix := []byte{
			escapedAlphabet[(i/9)%3],
			escapedAlphabet[(i/3)%3],
			escapedAlphabet[i%3],
		}
		models[i] = strings.Repeat("<", 252) + string(suffix)
	}
	sort.Strings(models)
	max := int64(math.MaxInt64)
	maxModel := strings.Repeat("<", 255)
	maxEffort := strings.Repeat("<", 64)
	maxVersion := strings.Repeat("<", 128)
	req.Attempt = math.MaxInt32
	req.ClaimGeneration = max
	req.Revision = max
	for _, timing := range []*TaskRunEvidenceTiming{
		&req.Timings.Queue, &req.Timings.Preparation, &req.Timings.FirstTool,
		&req.Timings.Execution, &req.Timings.Finalization,
	} {
		timing.Known = true
		timing.DurationMS = &max
	}
	req.Requested = TaskRunEvidenceModelConfig{Model: &maxModel, Effort: &maxEffort}
	req.ClientEffective = TaskRunEvidenceModelConfig{Model: &maxModel, Effort: &maxEffort}
	req.ProviderReported = TaskRunEvidenceProviderModel{Model: &maxModel, Source: "provider_summary"}
	req.Runtime = TaskRunEvidenceRuntime{Version: &maxVersion, ContentSHA256: req.Runtime.ContentSHA256}
	req.Usage = TaskRunEvidenceUsage{
		InputUncachedTokens: &max, InputCacheReadTokens: &max,
		InputCacheWriteTokens: &max, OutputTokens: &max,
		Complete: true, Source: "provider_summary",
	}
	req.ProviderCost = TaskRunEvidenceProviderCost{
		AmountUSDTicks: &max, Complete: true, Authority: "provider_reported",
		Basis: "provider_reported", Source: "provider_summary",
	}
	cost := TaskRunEvidenceProviderCost{
		AmountUSDTicks: &max, Complete: true, Authority: "provider_reported",
		Basis: "provider_reported", Source: "provider_summary",
	}
	usage := TaskRunEvidenceUsage{
		InputUncachedTokens: &max, InputCacheReadTokens: &max,
		InputCacheWriteTokens: &max, OutputTokens: &max,
		Complete: true, Source: "provider_summary",
	}
	req.ModelUsage = &TaskRunEvidenceModelUsageInventory{
		Entries: make([]TaskRunEvidenceModelUsage, 0, len(models)), Complete: true, Source: "model_usage",
	}
	for _, model := range models {
		req.ModelUsage.Entries = append(req.ModelUsage.Entries, TaskRunEvidenceModelUsage{Model: model, Usage: usage, ProviderCost: cost})
	}
	allEntries := req.ModelUsage.Entries
	req.ModelUsage.Entries = allEntries[:12]
	negotiatedBody, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(negotiatedBody) >= maxTaskRunEvidenceBodyBytes {
		t.Fatalf("12-entry negotiated maximum is %d bytes, limit is %d", len(negotiatedBody), maxTaskRunEvidenceBodyBytes)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(negotiatedBody))
	w := httptest.NewRecorder()
	if _, ok := decodeTaskRunEvidenceRequest(w, r); !ok {
		t.Fatalf("12-entry negotiated maximum rejected: status=%d body=%s", w.Code, w.Body.String())
	}

	req.ModelUsage.Entries = allEntries[:13]
	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("13-entry inventory exceeded negotiated maximum but was accepted")
	}

	req.ModelUsage.Entries = allEntries
	oversizedBody, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("maximal 12-entry body = %d bytes; maximal 16-entry body = %d bytes", len(negotiatedBody), len(oversizedBody))
	if len(oversizedBody) < maxTaskRunEvidenceBodyBytes {
		t.Fatalf("16-entry extreme body unexpectedly fits: body=%d limit=%d", len(oversizedBody), maxTaskRunEvidenceBodyBytes)
	}
	r = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(oversizedBody))
	w = httptest.NewRecorder()
	if _, ok := decodeTaskRunEvidenceRequest(w, r); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("oversized body accepted: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestValidateTaskRunEvidenceRejectsKnownTimingWithoutDuration(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.Timings.Queue.DurationMS = nil

	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTaskRunEvidenceCanonicalQueueTimingUsesDatabaseTimestamps(t *testing.T) {
	queuedAt := time.Date(2026, time.September, 6, 8, 0, 0, 123000000, time.UTC)
	dispatchedAt := queuedAt.Add(2750 * time.Millisecond)
	task := db.AgentTaskQueue{
		QueueStartedAt: pgtype.Timestamptz{Time: queuedAt, Valid: true},
		DispatchedAt:   pgtype.Timestamptz{Time: dispatchedAt, Valid: true},
	}

	got := taskRunEvidenceCanonicalQueueTiming(task)
	if !got.Known || got.DurationMS == nil || *got.DurationMS != 2750 {
		t.Fatalf("queue timing = %+v, want known 2750ms", got)
	}
}

func TestTaskRunEvidenceCanonicalQueueTimingKeepsHistoricalAndReclaimedClaimsUnknown(t *testing.T) {
	task := db.AgentTaskQueue{
		DispatchedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	got := taskRunEvidenceCanonicalQueueTiming(task)
	if got.Known || got.DurationMS != nil {
		t.Fatalf("queue timing = %+v, want unknown", got)
	}
}

func TestTaskRunEvidenceCanonicalQueueTimingRejectsNonFiniteAndZeroDatabaseTimes(t *testing.T) {
	finite := time.Date(2026, time.September, 6, 8, 0, 0, 0, time.UTC)
	tests := []db.AgentTaskQueue{
		{QueueStartedAt: pgtype.Timestamptz{Valid: true}, DispatchedAt: pgtype.Timestamptz{Time: finite, Valid: true}},
		{QueueStartedAt: pgtype.Timestamptz{Time: finite, Valid: true}, DispatchedAt: pgtype.Timestamptz{Valid: true}},
		{QueueStartedAt: pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, DispatchedAt: pgtype.Timestamptz{Time: finite, Valid: true}},
		{QueueStartedAt: pgtype.Timestamptz{Time: finite, Valid: true}, DispatchedAt: pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}},
	}
	for i, task := range tests {
		got := taskRunEvidenceCanonicalQueueTiming(task)
		if got.Known || got.DurationMS != nil {
			t.Fatalf("case %d queue timing = %+v, want unknown", i, got)
		}
	}
}

func TestTaskQueueStartedAtMigrationDoesNotBackfillHistoricalRows(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", "921_agent_task_queue_queue_started_at.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read queue timing migration: %v", err)
	}
	conn, err := testPool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire test database connection: %v", err)
	}
	defer conn.Release()
	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin migration test transaction: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `CREATE TEMP TABLE agent_task_queue (id INTEGER PRIMARY KEY); INSERT INTO agent_task_queue (id) VALUES (1)`); err != nil {
		t.Fatalf("create historical task row: %v", err)
	}
	if _, err := tx.Exec(context.Background(), string(migration)); err != nil {
		t.Fatalf("apply queue timing migration: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `INSERT INTO agent_task_queue (id) VALUES (2)`); err != nil {
		t.Fatalf("create post-migration task row: %v", err)
	}
	var historical, created pgtype.Timestamptz
	if err := tx.QueryRow(context.Background(), `SELECT queue_started_at FROM agent_task_queue WHERE id = 1`).Scan(&historical); err != nil {
		t.Fatalf("read historical queue anchor: %v", err)
	}
	if err := tx.QueryRow(context.Background(), `SELECT queue_started_at FROM agent_task_queue WHERE id = 2`).Scan(&created); err != nil {
		t.Fatalf("read new queue anchor: %v", err)
	}
	if historical.Valid || !created.Valid || created.InfinityModifier != pgtype.Finite || created.Time.IsZero() {
		t.Fatalf("queue anchors: historical=%+v new=%+v", historical, created)
	}
}

func TestTaskRunEvidenceModelUsageMigrationAddsNullableJSONB(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", "922_task_run_evidence_model_usage.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read model usage migration: %v", err)
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin migration test transaction: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `CREATE TEMP TABLE task_run_evidence (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create temporary evidence table: %v", err)
	}
	if _, err := tx.Exec(context.Background(), string(migration)); err != nil {
		t.Fatalf("apply model usage migration: %v", err)
	}
	var dataType, nullable string
	var defaultValue *string
	if err := tx.QueryRow(context.Background(), `
		SELECT data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema LIKE 'pg_temp_%'
		  AND table_name = 'task_run_evidence'
		  AND column_name = 'model_usage'
	`).Scan(&dataType, &nullable, &defaultValue); err != nil {
		t.Fatalf("inspect model usage column: %v", err)
	}
	if dataType != "jsonb" || nullable != "YES" || defaultValue != nil {
		t.Fatalf("model_usage column = type %q nullable %q default %v", dataType, nullable, defaultValue)
	}
}

func TestValidateTaskRunEvidenceRejectsMissingClaimGeneration(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.ClaimGeneration = 0

	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateTaskRunEvidencePreservesPartialUsageAsIncomplete(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.Usage.InputCacheWriteTokens = nil
	req.Usage.Complete = false

	if err := validateTaskRunEvidenceRequest(req); err != nil {
		t.Fatalf("validate partial usage: %v", err)
	}
}

func TestValidateTaskRunEvidenceAcceptsExplicitlyMissingUsage(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.Usage = TaskRunEvidenceUsage{Source: "missing"}

	if err := validateTaskRunEvidenceRequest(req); err != nil {
		t.Fatalf("validate missing usage: %v", err)
	}
}

func TestValidateTaskRunEvidenceRejectsCompleteUsageWithMissingBucket(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.Usage.InputCacheWriteTokens = nil

	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateTaskRunEvidenceRejectsHostPathInRuntimeVersion(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	version := "/opt/tools/codex-0.153.4"
	req.Runtime.Version = &version

	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateTaskRunEvidenceRejectsCompleteUnknownProviderCost(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.ProviderCost.Complete = true

	if err := validateTaskRunEvidenceRequest(req); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateTaskRunEvidenceAcceptsExplicitlyMissingProviderCost(t *testing.T) {
	req := validTaskRunEvidenceRequest()
	req.ProviderCost = TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"}

	if err := validateTaskRunEvidenceRequest(req); err != nil {
		t.Fatalf("validate missing provider cost: %v", err)
	}
}

func TestDecodeTaskRunEvidenceRejectsUnknownFields(t *testing.T) {
	body := []byte(`{"schema_version":"task_run_evidence/v1","attempt":1,"revision":1,"unknown":true}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	if _, ok := decodeTaskRunEvidenceRequest(w, req); ok {
		t.Fatal("expected unknown field rejection")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTaskRunEvidenceResponseDoesNotExposeAttestedIdentity(t *testing.T) {
	response := TaskRunEvidenceResponse{SchemaVersion: taskRunEvidenceSchemaVersion}

	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	for _, forbidden := range []string{"workspace_id", "runtime_id", "daemon_id", "pid", "argv", "env", "executable_path"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("response contains forbidden field %q: %s", forbidden, raw)
		}
	}
	if bytes.Contains(raw, []byte(`"claim_generation":`)) {
		t.Fatalf("response exposes raw claim generation: %s", raw)
	}
}

type taskRunEvidenceFixture struct {
	Handler   *Handler
	Tx        pgx.Tx
	RuntimeID string
	TaskID    string
	DaemonID  string
}

func newTaskRunEvidenceFixture(t *testing.T) taskRunEvidenceFixture {
	t.Helper()
	daemonID := "task-run-evidence-daemon"
	runtimeID := dbfx.Runtime(t, "task run evidence runtime", testutil.Cols{"daemon_id": daemonID})
	agentID := dbfx.Agent(t, "task run evidence agent", runtimeID)
	issueID := dbfx.Issue(t, "task run evidence issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":         issueID,
		"runtime_id":       runtimeID,
		"status":           "running",
		"attempt":          2,
		"queue_started_at": time.UnixMicro(taskRunEvidenceTestClaimGeneration).UTC().Add(-1500 * time.Millisecond),
		"dispatched_at":    time.UnixMicro(taskRunEvidenceTestClaimGeneration).UTC(),
	})

	conn, err := testPool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire test database connection: %v", err)
	}
	tx, err := conn.Begin(context.Background())
	if err != nil {
		conn.Release()
		t.Fatalf("begin test database transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
		conn.Release()
	})

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", "911_task_run_evidence.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read task run evidence migration: %v", err)
	}
	tempDDL := strings.Replace(string(migration), "CREATE TABLE task_run_evidence", "CREATE TEMP TABLE task_run_evidence", 1)
	if _, err := tx.Exec(context.Background(), tempDDL); err != nil {
		t.Fatalf("create temporary task run evidence table: %v", err)
	}
	modelUsageMigrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations", "922_task_run_evidence_model_usage.up.sql")
	modelUsageMigration, err := os.ReadFile(modelUsageMigrationPath)
	if err != nil {
		t.Fatalf("read task run evidence model usage migration: %v", err)
	}
	if _, err := tx.Exec(context.Background(), string(modelUsageMigration)); err != nil {
		t.Fatalf("add temporary task run evidence model usage: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `ALTER TABLE task_run_evidence ADD COLUMN claim_generation BIGINT CHECK (claim_generation > 0)`); err != nil {
		t.Fatalf("add temporary task run evidence claim generation: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `CREATE UNIQUE INDEX task_run_evidence_test_task_attempt_generation ON task_run_evidence (task_id, attempt, claim_generation) NULLS NOT DISTINCT`); err != nil {
		t.Fatalf("create temporary task run evidence index: %v", err)
	}
	var hasConciseMode bool
	if err := tx.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'agent_task_queue' AND column_name = 'concise_mode'
		)
	`).Scan(&hasConciseMode); err != nil {
		t.Fatalf("inspect agent_task_queue schema: %v", err)
	}
	if !hasConciseMode {
		if _, err := tx.Exec(context.Background(), `CREATE TEMP VIEW agent_task_queue AS SELECT q.*, false AS concise_mode FROM public.agent_task_queue q`); err != nil {
			t.Fatalf("create temporary agent_task_queue compatibility view: %v", err)
		}
	}

	h := *testHandler
	h.Queries = db.New(tx)
	h.TxStarter = tx
	return taskRunEvidenceFixture{Handler: &h, Tx: tx, RuntimeID: runtimeID, TaskID: taskID, DaemonID: daemonID}
}

func (f taskRunEvidenceFixture) post(t *testing.T, req TaskRunEvidenceRequest, daemonID, runtimeID string) *testutil.Response {
	t.Helper()
	request := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/tasks/"+f.TaskID+"/run-evidence", req, testWorkspaceID, daemonID)
	request = withURLParams(request, "runtimeId", runtimeID, "taskId", f.TaskID)
	return testutil.Call(t, f.Handler.ReportTaskRunEvidence, request)
}

func TestReportTaskRunEvidenceRoundTripsThroughUserRead(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	post := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, f.RuntimeID)
	if post.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", post.Code, post.Body.String())
	}

	request := newRequest(http.MethodGet, "/api/tasks/"+f.TaskID+"/run-evidence", nil)
	request = withURLParam(request, "taskId", f.TaskID)
	request = request.WithContext(middleware.SetMemberContext(request.Context(), testWorkspaceID, db.Member{}))
	response := testutil.Call(t, f.Handler.ListTaskRunEvidenceByUser, request).Want(http.StatusOK)
	var body TaskRunEvidenceListResponse
	response.JSON(&body)
	if len(body.Attempts) != 1 || body.Attempts[0].Usage.InputCacheWriteTokens == nil || *body.Attempts[0].Usage.InputCacheWriteTokens != 5 {
		t.Fatalf("unexpected round-trip response: %+v", body)
	}
	if body.Attempts[0].EvidenceID == nil || !taskRunEvidenceSHA256Pattern.MatchString(*body.Attempts[0].EvidenceID) || body.Attempts[0].ClaimIdentitySource != "claim_generation" {
		t.Fatalf("claim identity = id %v source %q", body.Attempts[0].EvidenceID, body.Attempts[0].ClaimIdentitySource)
	}
	if !body.Attempts[0].Timings.Queue.Known || body.Attempts[0].Timings.Queue.DurationMS == nil || *body.Attempts[0].Timings.Queue.DurationMS != 1500 {
		t.Fatalf("queue timing = %+v, want server-derived 1500ms", body.Attempts[0].Timings.Queue)
	}
}

func TestReportTaskRunEvidenceCanonicalizesQueueBeforeHashing(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	first := validTaskRunEvidenceRequest()
	first.Timings.Queue = TaskRunEvidenceTiming{}
	if got := f.post(t, first, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("first post status = %d: %s", got.Code, got.Body.String())
	}

	second := validTaskRunEvidenceRequest()
	clientValue := int64(999999)
	second.Timings.Queue = TaskRunEvidenceTiming{Known: true, DurationMS: &clientValue}
	if got := f.post(t, second, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("canonical-equivalent post status = %d: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceKeepsMissingQueueAnchorUnknown(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	if _, err := f.Tx.Exec(context.Background(), `UPDATE agent_task_queue SET queue_started_at = NULL WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("clear queue anchor: %v", err)
	}
	req := validTaskRunEvidenceRequest()
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", got.Code, got.Body.String())
	}

	rows, err := f.Handler.Queries.ListTaskRunEvidence(context.Background(), parseUUID(f.TaskID))
	if err != nil || len(rows) != 1 {
		t.Fatalf("list evidence: rows=%d err=%v", len(rows), err)
	}
	if rows[0].QueueKnown || rows[0].QueueDurationMs.Valid {
		t.Fatalf("queue timing = known %v duration %v, want unknown", rows[0].QueueKnown, rows[0].QueueDurationMs)
	}
}

func TestTaskQueueStartedAtResetsOnRequeueAndClearsOnStaleReclaim(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "dispatched")
	ctx := context.Background()
	oldAnchor := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET queue_started_at = $2 WHERE id = $1`, f.TaskID, oldAnchor); err != nil {
		t.Fatalf("set old queue anchor: %v", err)
	}
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(f.TaskID))
	if err != nil {
		t.Fatalf("load dispatched task: %v", err)
	}
	requeued, err := testHandler.Queries.RequeueAgentTaskAfterClaimFailure(ctx, db.RequeueAgentTaskAfterClaimFailureParams{
		TaskID: task.ID, RuntimeID: task.RuntimeID, DispatchedAt: task.DispatchedAt,
	})
	if err != nil {
		t.Fatalf("requeue task: %v", err)
	}
	if !requeued.QueueStartedAt.Valid || !requeued.QueueStartedAt.Time.After(oldAnchor) {
		t.Fatalf("requeued anchor = %+v, want fresh database timestamp", requeued.QueueStartedAt)
	}

	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'online', last_seen_at = now() WHERE id = $1`, f.RuntimeID); err != nil {
		t.Fatalf("prepare stale runtime: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_task_queue
		SET status = 'dispatched', dispatched_at = now() - interval '10 minutes',
			queue_started_at = now() - interval '11 minutes', prepare_lease_expires_at = NULL
		WHERE id = $1
	`, f.TaskID); err != nil {
		t.Fatalf("prepare stale claim: %v", err)
	}
	reclaimed, err := testHandler.Queries.ReclaimStaleDispatchedTaskForRuntime(ctx, db.ReclaimStaleDispatchedTaskForRuntimeParams{
		RuntimeID: parseUUID(f.RuntimeID), ClaimRecoverySecs: 1, PrepareLeaseSecs: 30, RuntimeStaleSecs: 3600,
	})
	if err != nil {
		t.Fatalf("reclaim stale task: %v", err)
	}
	if reclaimed.QueueStartedAt.Valid {
		t.Fatalf("reclaimed anchor = %+v, want unknown", reclaimed.QueueStartedAt)
	}
}

func TestReportTaskRunEvidenceRetainsEachClaimGenerationForSameAttempt(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	first := validTaskRunEvidenceRequest()
	first.Revision = 9
	if got := f.post(t, first, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("first claim post status = %d: %s", got.Code, got.Body.String())
	}

	secondGeneration := taskRunEvidenceTestClaimGeneration + 1
	if _, err := f.Tx.Exec(context.Background(), `UPDATE agent_task_queue SET dispatched_at = $2 WHERE id = $1`, f.TaskID, time.UnixMicro(secondGeneration).UTC()); err != nil {
		t.Fatalf("advance claim generation: %v", err)
	}
	second := validTaskRunEvidenceRequest()
	second.ClaimGeneration = secondGeneration
	if got := f.post(t, second, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("second claim post status = %d: %s", got.Code, got.Body.String())
	}

	rows, err := f.Handler.Queries.ListTaskRunEvidence(context.Background(), parseUUID(f.TaskID))
	if err != nil {
		t.Fatalf("list claim evidence: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("evidence rows = %d, want 2", len(rows))
	}
}

func TestTaskRunEvidenceClaimIdentityDiffersAcrossClaimGenerations(t *testing.T) {
	row := db.TaskRunEvidence{
		TaskID:          parseUUID("00000000-0000-0000-0000-000000000123"),
		Attempt:         2,
		ClaimGeneration: pgtype.Int8{Int64: 100, Valid: true},
	}
	first, _ := taskRunEvidenceClaimIdentity(row)
	row.ClaimGeneration.Int64 = 101
	second, _ := taskRunEvidenceClaimIdentity(row)

	if first == nil || second == nil || *first == *second {
		t.Fatalf("claim identities = %v and %v, want distinct opaque ids", first, second)
	}
}

func TestTaskRunEvidenceResponseMarksLegacyClaimIdentityMissing(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	if got := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", got.Code, got.Body.String())
	}
	if _, err := f.Tx.Exec(context.Background(), `UPDATE task_run_evidence SET claim_generation = NULL WHERE task_id = $1`, f.TaskID); err != nil {
		t.Fatalf("convert row to legacy identity: %v", err)
	}
	rows, err := f.Handler.Queries.ListTaskRunEvidence(context.Background(), parseUUID(f.TaskID))
	if err != nil || len(rows) != 1 {
		t.Fatalf("list legacy evidence: rows=%d err=%v", len(rows), err)
	}
	response := taskRunEvidenceResponse(rows[0])
	if response.EvidenceID != nil || response.ClaimIdentitySource != "missing" {
		t.Fatalf("legacy claim identity = id %v source %q", response.EvidenceID, response.ClaimIdentitySource)
	}
}

func TestReportTaskRunEvidenceIsIdempotentForSameRevisionAndPayload(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("first post status = %d: %s", got.Code, got.Body.String())
	}
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("second post status = %d: %s", got.Code, got.Body.String())
	}
	var count int
	if err := f.Tx.QueryRow(context.Background(), `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.TaskID).Scan(&count); err != nil {
		t.Fatalf("count evidence rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("evidence row count = %d, want 1", count)
	}
}

func TestReportTaskRunEvidenceRejectsOlderRevision(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	req.Revision = 2
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("initial post status = %d: %s", got.Code, got.Body.String())
	}
	req.Revision = 1
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusConflict {
		t.Fatalf("older revision status = %d, want 409: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceUpdatesToHigherRevision(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("initial post status = %d: %s", got.Code, got.Body.String())
	}
	req.Revision = 2
	changed := int64(11)
	req.Usage.OutputTokens = &changed
	got := f.post(t, req, f.DaemonID, f.RuntimeID)
	if got.Code != http.StatusOK {
		t.Fatalf("higher revision status = %d: %s", got.Code, got.Body.String())
	}
	var response TaskRunEvidenceResponse
	got.JSON(&response)
	if response.Revision != 2 {
		t.Fatalf("revision = %d, want 2", response.Revision)
	}
}

func TestReportTaskRunEvidenceModelUsageNullableRoundTripAndHigherRevision(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	initial := validTaskRunEvidenceRequest()
	got := f.post(t, initial, f.DaemonID, f.RuntimeID)
	if got.Code != http.StatusOK {
		t.Fatalf("initial post status = %d: %s", got.Code, got.Body.String())
	}
	var response TaskRunEvidenceResponse
	got.JSON(&response)
	if response.ModelUsage != nil {
		t.Fatalf("legacy evidence gained model_usage: %+v", response.ModelUsage)
	}
	if bytes.Contains(got.Body.Bytes(), []byte(`"model_usage":`)) {
		t.Fatalf("legacy evidence response did not omit model_usage: %s", got.Body.String())
	}

	updated := validTaskRunEvidenceRequest()
	updated.Revision = 2
	updated.ModelUsage = completeTaskRunEvidenceModelUsage()
	got = f.post(t, updated, f.DaemonID, f.RuntimeID)
	if got.Code != http.StatusOK {
		t.Fatalf("higher revision status = %d: %s", got.Code, got.Body.String())
	}
	got.JSON(&response)
	if response.Revision != 2 || response.ModelUsage == nil || len(response.ModelUsage.Entries) != 2 ||
		response.ModelUsage.Entries[0].Model != "a-model" || !response.ModelUsage.Complete || response.ModelUsage.Truncated {
		t.Fatalf("model usage round trip = %+v", response.ModelUsage)
	}
}

func TestReportTaskRunEvidencePreservesProviderCostTicksExactly(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	got := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, f.RuntimeID)
	if got.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", got.Code, got.Body.String())
	}
	var response TaskRunEvidenceResponse
	got.JSON(&response)
	if response.ProviderCost.AmountUSDTicks == nil || *response.ProviderCost.AmountUSDTicks != 19990000000 {
		t.Fatalf("provider cost ticks = %v, want 19990000000", response.ProviderCost.AmountUSDTicks)
	}
}

func TestReportTaskRunEvidenceRejectsConflictingSameRevision(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("initial post status = %d: %s", got.Code, got.Body.String())
	}
	changed := int64(11)
	req.Usage.OutputTokens = &changed
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusConflict {
		t.Fatalf("conflicting revision status = %d, want 409: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsWrongDaemon(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	if got := f.post(t, validTaskRunEvidenceRequest(), "different-daemon", f.RuntimeID); got.Code != http.StatusNotFound {
		t.Fatalf("wrong daemon status = %d, want 404: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsWrongRuntime(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	otherRuntimeID := dbfx.Runtime(t, "other evidence runtime", testutil.Cols{"daemon_id": f.DaemonID, "provider": "other_evidence_runtime"})
	if got := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, otherRuntimeID); got.Code != http.StatusNotFound {
		t.Fatalf("wrong runtime status = %d, want 404: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsWrongWorkspace(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	request := newDaemonTokenRequest(http.MethodPost, "/", validTaskRunEvidenceRequest(), "00000000-0000-0000-0000-000000000001", f.DaemonID)
	request = withURLParams(request, "runtimeId", f.RuntimeID, "taskId", f.TaskID)
	if got := testutil.Call(t, f.Handler.ReportTaskRunEvidence, request); got.Code != http.StatusNotFound {
		t.Fatalf("wrong workspace status = %d, want 404", got.Code)
	}
}

func TestReportTaskRunEvidenceRejectsStaleAttempt(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	req.Attempt = 1
	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusConflict {
		t.Fatalf("stale attempt status = %d, want 409: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsStaleClaimGeneration(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	req := validTaskRunEvidenceRequest()
	req.ClaimGeneration++

	if got := f.post(t, req, f.DaemonID, f.RuntimeID); got.Code != http.StatusConflict {
		t.Fatalf("stale claim generation status = %d, want 409: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceAcceptsPreparationFailureWhileDispatched(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "dispatched")
	req := validTaskRunEvidenceRequest()
	req.Revision = 2
	req.Timings.Execution = TaskRunEvidenceTiming{}
	req.Timings.Finalization = TaskRunEvidenceTiming{}

	if got := testutil.Call(t, testHandler.ReportTaskRunEvidence, publicTaskRunEvidenceRequest(f, req)); got.Code != http.StatusOK {
		t.Fatalf("dispatched preparation evidence status = %d, want 200: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceAcceptsTerminalRevisionAfterCancellation(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "running")
	if _, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET status = 'cancelled', completed_at = now() WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	req := validTaskRunEvidenceRequest()
	req.Revision = 5

	if got := testutil.Call(t, testHandler.ReportTaskRunEvidence, publicTaskRunEvidenceRequest(f, req)); got.Code != http.StatusOK {
		t.Fatalf("cancelled terminal evidence status = %d, want 200: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsRequeuedTaskWithoutClaimGeneration(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "running")
	if _, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET status = 'queued', dispatched_at = NULL WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("requeue task: %v", err)
	}

	if got := testutil.Call(t, testHandler.ReportTaskRunEvidence, publicTaskRunEvidenceRequest(f, validTaskRunEvidenceRequest())); got.Code != http.StatusConflict {
		t.Fatalf("requeued evidence status = %d, want 409: %s", got.Code, got.Body.String())
	}
}

func TestReportTaskRunEvidenceRejectsUserAuthWithoutDaemonAttestation(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	request := newRequest(http.MethodPost, "/", validTaskRunEvidenceRequest())
	request = withURLParams(request, "runtimeId", f.RuntimeID, "taskId", f.TaskID)
	if got := testutil.Call(t, f.Handler.ReportTaskRunEvidence, request); got.Code != http.StatusForbidden {
		t.Fatalf("user-authenticated post status = %d, want 403", got.Code)
	}
}

func TestListTaskRunEvidenceByUserRejectsForeignWorkspace(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	request := newRequest(http.MethodGet, "/", nil)
	request = withURLParam(request, "taskId", f.TaskID)
	request = request.WithContext(middleware.SetMemberContext(request.Context(), "00000000-0000-0000-0000-000000000001", db.Member{}))
	if got := testutil.Call(t, f.Handler.ListTaskRunEvidenceByUser, request); got.Code != http.StatusNotFound {
		t.Fatalf("foreign workspace read status = %d, want 404", got.Code)
	}
}

func TestWorkspaceDeletionManifestClassifiesTaskRunEvidenceForDeletion(t *testing.T) {
	if got := workspaceDeletionManifest["task_run_evidence"]; got != workspaceDelete {
		t.Fatalf("task_run_evidence action = %q, want %q", got, workspaceDelete)
	}
}

func TestDeleteTaskBatchDeletesTaskRunEvidence(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	if got := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", got.Code, got.Body.String())
	}

	if err := f.Handler.Queries.DeleteTaskBatch(context.Background(), []pgtype.UUID{parseUUID(f.TaskID)}); err != nil {
		t.Fatalf("delete task batch: %v", err)
	}
	assertTaskAndEvidenceDeleted(t, f)
}

func TestDeleteUnstartedQuickCreateRetryTaskDeletesTaskRunEvidence(t *testing.T) {
	f := newTaskRunEvidenceFixture(t)
	if got := f.post(t, validTaskRunEvidenceRequest(), f.DaemonID, f.RuntimeID); got.Code != http.StatusOK {
		t.Fatalf("post status = %d: %s", got.Code, got.Body.String())
	}
	if _, err := f.Tx.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'queued', issue_id = NULL, chat_session_id = NULL, autopilot_run_id = NULL
		WHERE id = $1
	`, f.TaskID); err != nil {
		t.Fatalf("prepare quick-create retry task: %v", err)
	}

	deleted, err := f.Handler.Queries.DeleteUnstartedQuickCreateRetryTask(context.Background(), parseUUID(f.TaskID))
	if err != nil {
		t.Fatalf("delete unstarted quick-create retry task: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted tasks = %d, want 1", deleted)
	}
	assertTaskAndEvidenceDeleted(t, f)
}

func assertTaskAndEvidenceDeleted(t *testing.T, f taskRunEvidenceFixture) {
	t.Helper()
	var taskCount, evidenceCount int
	if err := f.Tx.QueryRow(context.Background(), `SELECT count(*) FROM agent_task_queue WHERE id = $1`, f.TaskID).Scan(&taskCount); err != nil {
		t.Fatalf("count task rows: %v", err)
	}
	if err := f.Tx.QueryRow(context.Background(), `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.TaskID).Scan(&evidenceCount); err != nil {
		t.Fatalf("count evidence rows: %v", err)
	}
	if taskCount != 0 || evidenceCount != 0 {
		t.Fatalf("remaining rows: task=%d evidence=%d", taskCount, evidenceCount)
	}
}

type publicTaskRunEvidenceFixture struct {
	RuntimeID       string
	TaskID          string
	DaemonID        string
	ClaimGeneration int64
}

func newPublicTaskRunEvidenceFixture(t *testing.T, status string) publicTaskRunEvidenceFixture {
	t.Helper()
	daemonID := "task-run-evidence-race-daemon"
	runtimeID := dbfx.Runtime(t, "task run evidence race runtime", testutil.Cols{"daemon_id": daemonID})
	agentID := dbfx.Agent(t, "task run evidence race agent", runtimeID)
	issueID := dbfx.Issue(t, "task run evidence race issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"issue_id":      issueID,
		"runtime_id":    runtimeID,
		"status":        status,
		"attempt":       2,
		"dispatched_at": time.UnixMicro(taskRunEvidenceTestClaimGeneration).UTC(),
	})
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM task_run_evidence WHERE task_id = $1`, taskID)
	})
	return publicTaskRunEvidenceFixture{RuntimeID: runtimeID, TaskID: taskID, DaemonID: daemonID, ClaimGeneration: taskRunEvidenceTestClaimGeneration}
}

func publicTaskRunEvidenceRequest(f publicTaskRunEvidenceFixture, req TaskRunEvidenceRequest) *http.Request {
	r := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+f.RuntimeID+"/tasks/"+f.TaskID+"/run-evidence", req, testWorkspaceID, f.DaemonID)
	return withURLParams(r, "runtimeId", f.RuntimeID, "taskId", f.TaskID)
}

func callTaskRunEvidenceAsync(f publicTaskRunEvidenceFixture, req TaskRunEvidenceRequest) <-chan *httptest.ResponseRecorder {
	done := make(chan *httptest.ResponseRecorder, 1)
	r := publicTaskRunEvidenceRequest(f, req)
	go func() {
		w := httptest.NewRecorder()
		testHandler.ReportTaskRunEvidence(w, r)
		done <- w
	}()
	return done
}

func TestDeleteUnstartedQuickCreateRetryTaskDoesNotDeleteConcurrentlyStartedTask(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "queued")
	if _, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET issue_id = NULL WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("prepare quick-create task: %v", err)
	}

	ctx := context.Background()
	starter, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin starter transaction: %v", err)
	}
	defer starter.Rollback(context.Background())
	holderPID := holderBackendPID(t, ctx, starter)
	if _, err := starter.Exec(ctx, `SELECT id FROM agent_task_queue WHERE id = $1 FOR UPDATE`, f.TaskID); err != nil {
		t.Fatalf("lock task for start: %v", err)
	}

	deleted := make(chan struct {
		count int64
		err   error
	}, 1)
	go func() {
		count, deleteErr := testHandler.Queries.DeleteUnstartedQuickCreateRetryTask(context.Background(), parseUUID(f.TaskID))
		deleted <- struct {
			count int64
			err   error
		}{count: count, err: deleteErr}
	}()
	if !waitForWaiterBlockedBy(t, holderPID, 10*time.Second) {
		t.Fatal("retry cleanup did not wait for the task row lock")
	}
	if _, err := starter.Exec(ctx, `UPDATE agent_task_queue SET status = 'running' WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("start task: %v", err)
	}
	if err := starter.Commit(ctx); err != nil {
		t.Fatalf("commit task start: %v", err)
	}

	result := <-deleted
	if result.err != nil {
		t.Fatalf("delete unstarted retry: %v", result.err)
	}
	if result.count != 0 {
		t.Fatalf("deleted tasks = %d, want 0 after concurrent start", result.count)
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM agent_task_queue WHERE id = $1`, f.TaskID).Scan(&status); err != nil {
		t.Fatalf("reload concurrently started task: %v", err)
	}
	if status != "running" {
		t.Fatalf("task status = %q, want running", status)
	}
}

func TestDeleteUnstartedQuickCreateRetryTaskDoesNotDeleteConcurrentlyAttachedTask(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "queued")
	if _, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET issue_id = NULL WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("prepare quick-create task: %v", err)
	}
	issueID := dbfx.Issue(t, "concurrent retry attachment")

	ctx := context.Background()
	attacher, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin attachment transaction: %v", err)
	}
	defer attacher.Rollback(context.Background())
	holderPID := holderBackendPID(t, ctx, attacher)
	if _, err := attacher.Exec(ctx, `SELECT id FROM agent_task_queue WHERE id = $1 FOR UPDATE`, f.TaskID); err != nil {
		t.Fatalf("lock task for attachment: %v", err)
	}

	deleted := make(chan struct {
		count int64
		err   error
	}, 1)
	go func() {
		count, deleteErr := testHandler.Queries.DeleteUnstartedQuickCreateRetryTask(context.Background(), parseUUID(f.TaskID))
		deleted <- struct {
			count int64
			err   error
		}{count: count, err: deleteErr}
	}()
	if !waitForWaiterBlockedBy(t, holderPID, 10*time.Second) {
		t.Fatal("retry cleanup did not wait for the task row lock")
	}
	if _, err := attacher.Exec(ctx, `UPDATE agent_task_queue SET issue_id = $2 WHERE id = $1`, f.TaskID, issueID); err != nil {
		t.Fatalf("attach task context: %v", err)
	}
	if err := attacher.Commit(ctx); err != nil {
		t.Fatalf("commit task attachment: %v", err)
	}

	result := <-deleted
	if result.err != nil {
		t.Fatalf("delete unstarted retry: %v", result.err)
	}
	if result.count != 0 {
		t.Fatalf("deleted tasks = %d, want 0 after concurrent attachment", result.count)
	}
	var attachedIssueID string
	if err := testPool.QueryRow(ctx, `SELECT issue_id::text FROM agent_task_queue WHERE id = $1`, f.TaskID).Scan(&attachedIssueID); err != nil {
		t.Fatalf("reload concurrently attached task: %v", err)
	}
	if attachedIssueID != issueID {
		t.Fatalf("task issue_id = %q, want %q", attachedIssueID, issueID)
	}
}

func TestReportTaskRunEvidenceDoesNotInsertAfterConcurrentTaskDelete(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "running")
	ctx := context.Background()
	deleter, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin delete transaction: %v", err)
	}
	defer deleter.Rollback(context.Background())
	holderPID := holderBackendPID(t, ctx, deleter)
	if _, err := deleter.Exec(ctx, `SELECT id FROM agent_task_queue WHERE id = $1 FOR UPDATE`, f.TaskID); err != nil {
		t.Fatalf("lock task for delete: %v", err)
	}
	if _, err := deleter.Exec(ctx, `DELETE FROM task_run_evidence WHERE task_id = $1`, f.TaskID); err != nil {
		t.Fatalf("clean task evidence before delete: %v", err)
	}

	done := callTaskRunEvidenceAsync(f, validTaskRunEvidenceRequest())
	if !waitForWaiterBlockedBy(t, holderPID, 10*time.Second) {
		select {
		case response := <-done:
			t.Fatalf("evidence write completed with status %d before the task delete committed", response.Code)
		default:
			t.Fatal("evidence write neither blocked on the task row nor completed")
		}
	}
	if _, err := deleter.Exec(ctx, `DELETE FROM agent_task_queue WHERE id = $1`, f.TaskID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if err := deleter.Commit(ctx); err != nil {
		t.Fatalf("commit task delete: %v", err)
	}

	response := <-done
	if response.Code != http.StatusNotFound {
		t.Fatalf("post-delete evidence status = %d, want 404: %s", response.Code, response.Body.String())
	}
	var evidenceCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.TaskID).Scan(&evidenceCount); err != nil {
		t.Fatalf("count orphan evidence: %v", err)
	}
	if evidenceCount != 0 {
		t.Fatalf("orphan evidence rows = %d, want 0", evidenceCount)
	}
}

func TestReportTaskRunEvidenceRejectsSameAttemptReclaimedWhileWriteWaits(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "running")
	ctx := context.Background()
	reclaimer, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin reclaim transaction: %v", err)
	}
	defer reclaimer.Rollback(context.Background())
	holderPID := holderBackendPID(t, ctx, reclaimer)
	if _, err := reclaimer.Exec(ctx, `SELECT id FROM agent_task_queue WHERE id = $1 FOR UPDATE`, f.TaskID); err != nil {
		t.Fatalf("lock task for reclaim: %v", err)
	}

	done := callTaskRunEvidenceAsync(f, validTaskRunEvidenceRequest())
	if !waitForWaiterBlockedBy(t, holderPID, 10*time.Second) {
		select {
		case response := <-done:
			t.Fatalf("evidence write completed with status %d before attempt reclaim committed", response.Code)
		default:
			t.Fatal("evidence write neither blocked on the task row nor completed")
		}
	}
	if _, err := reclaimer.Exec(ctx, `UPDATE agent_task_queue SET dispatched_at = $2 WHERE id = $1`, f.TaskID, time.UnixMicro(f.ClaimGeneration+1).UTC()); err != nil {
		t.Fatalf("reclaim task generation: %v", err)
	}
	if err := reclaimer.Commit(ctx); err != nil {
		t.Fatalf("commit claim generation change: %v", err)
	}

	response := <-done
	if response.Code != http.StatusConflict {
		t.Fatalf("stale attempt status = %d, want 409: %s", response.Code, response.Body.String())
	}
	var evidenceCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.TaskID).Scan(&evidenceCount); err != nil {
		t.Fatalf("count stale evidence: %v", err)
	}
	if evidenceCount != 0 {
		t.Fatalf("stale claim evidence rows = %d, want 0", evidenceCount)
	}
}

func TestReportTaskRunEvidenceRejectsRuntimeUnboundWhileWriteWaits(t *testing.T) {
	f := newPublicTaskRunEvidenceFixture(t, "running")
	ctx := context.Background()
	unbinder, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin runtime unbind transaction: %v", err)
	}
	defer unbinder.Rollback(context.Background())
	holderPID := holderBackendPID(t, ctx, unbinder)
	if _, err := unbinder.Exec(ctx, `UPDATE agent_runtime SET daemon_id = $2 WHERE id = $1`, f.RuntimeID, "replacement-daemon"); err != nil {
		t.Fatalf("unbind runtime: %v", err)
	}

	done := callTaskRunEvidenceAsync(f, validTaskRunEvidenceRequest())
	if !waitForWaiterBlockedBy(t, holderPID, 10*time.Second) {
		select {
		case response := <-done:
			t.Fatalf("evidence write completed with status %d before runtime unbind committed", response.Code)
		default:
			t.Fatal("evidence write neither blocked on the runtime row nor completed")
		}
	}
	if err := unbinder.Commit(ctx); err != nil {
		t.Fatalf("commit runtime unbind: %v", err)
	}

	response := <-done
	if response.Code != http.StatusNotFound {
		t.Fatalf("post-unbind evidence status = %d, want 404: %s", response.Code, response.Body.String())
	}
	var evidenceCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.TaskID).Scan(&evidenceCount); err != nil {
		t.Fatalf("count stale runtime evidence: %v", err)
	}
	if evidenceCount != 0 {
		t.Fatalf("stale runtime evidence rows = %d, want 0", evidenceCount)
	}
}
