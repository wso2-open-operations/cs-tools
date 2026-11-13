package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestRegistrationValidationWritesOnlyWhatTheJobOwns(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewValidationRepository(pool)

	const company = "validation-agent-co"
	registered := time.Now().Add(-time.Hour)
	if _, err := NewIngestRepository(pool).Register(ctx, domain.Registration{
		CompanyID: company, UserID: "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5c",
		InitiatingProduct: "agent-manager", CreatedOn: &domain.SourceTime{Time: registered},
	}, nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	// No registration playbook yet: the write is refused and changes nothing.
	items, err := repo.Queue(ctx, "AGENT_PLATFORM", registered.Add(-time.Minute), 100)
	it, ok := queued(items, company)
	if err != nil || !ok {
		t.Fatalf("queue = %+v, %v; want %s in it", items, err, company)
	}
	pairing := it.OrgPlatformID
	if it.EmailClassification != nil || it.LastValidatedOn != nil {
		t.Errorf("queue item %+v", it)
	}
	var nf *apierror.NotFoundError
	if _, err := repo.Apply(ctx, applyReq(pairing)); !errors.As(err, &nf) {
		t.Fatalf("apply without a playbook: err = %v, want not found", err)
	}

	seedRegistrationPlaybook(t, "AGENT_PLATFORM")

	// First write attaches the playbook and fills the tasks.
	res, err := repo.Apply(ctx, applyReq(pairing))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Attached || len(res.Written) != 4 || len(res.Skipped) != 1 || res.Skipped[0].Reason != domain.SkipMissing || res.Abandoned {
		t.Errorf("first apply = %+v; want attached, 4 written, NO_SUCH_TASK skipped as missing, not abandoned", res)
	}
	var source, by *string
	var done *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT value_source::TEXT, completed_on, completed_by::TEXT
		FROM   plg_playbook_run_task WHERE playbook_run_id = $1::UUID AND code = 'ONBOARDING_STATUS'`,
		res.PlaybookRunID).Scan(&source, &done, &by); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if source == nil || *source != domain.ValueSourceValidation || done == nil || by != nil {
		t.Errorf("ONBOARDING_STATUS source=%v completed_on=%v completed_by=%v; want VALIDATION, stamped, no user", source, done, by)
	}
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT event_count FROM plg_activity_summary WHERE org_platform_id = $1::UUID`, pairing).Scan(&count); err != nil || count != 7 {
		t.Errorf("summary event_count = %d, %v; want 7", count, err)
	}

	// An engineer's value is never overwritten; the run is reused.
	if _, err := pool.Exec(ctx, `
		UPDATE plg_playbook_run_task SET value_text = 'PERSONAL', value_source = 'MANUAL'
		WHERE  playbook_run_id = $1::UUID AND code = $2`, res.PlaybookRunID, domain.TaskCodeEmailClassification); err != nil {
		t.Fatalf("engineer write: %v", err)
	}
	res2, err := repo.Apply(ctx, applyReq(pairing))
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if res2.Attached || res2.PlaybookRunID != res.PlaybookRunID || !skipped(res2, domain.TaskCodeEmailClassification, domain.SkipManual) {
		t.Errorf("second apply = %+v; want same run, classification skipped as MANUAL", res2)
	}
	items, _ = repo.Queue(ctx, "AGENT_PLATFORM", registered.Add(-time.Minute), 100)
	if it, ok := queued(items, company); !ok || it.EmailClassification == nil || *it.EmailClassification != "PERSONAL" || it.LastValidatedOn == nil {
		t.Errorf("queue after writes = %+v; want PERSONAL and a last-validated time", it)
	}

	// A value the task cannot hold rolls the whole write back.
	bad := applyReq(pairing)
	bad.Summary.EventCount = 99
	wrong := "NOT_AN_OPTION"
	bad.Values = []domain.ValidationValue{{Code: "ONBOARDING_STATUS", TextValue: &wrong}}
	var ve *apierror.ValidationError
	if _, err := repo.Apply(ctx, bad); !errors.As(err, &ve) {
		t.Errorf("bad option: err = %v, want a validation error", err)
	}
	_ = pool.QueryRow(ctx, `SELECT event_count FROM plg_activity_summary WHERE org_platform_id = $1::UUID`, pairing).Scan(&count)
	if count != 7 {
		t.Errorf("a refused write changed the summary to %d", count)
	}

	// A closed run is left as the engineer closed it.
	if _, err := pool.Exec(ctx, `
		UPDATE plg_playbook_run_task SET value_bool = TRUE, value_source = 'MANUAL'
		WHERE  playbook_run_id = $1::UUID AND code = $2`, res.PlaybookRunID, domain.TaskCodeClose); err != nil {
		t.Fatalf("close run: %v", err)
	}
	res3, err := repo.Apply(ctx, applyReq(pairing))
	if err != nil || !skipped(res3, "ONBOARDING_STATUS", domain.SkipClosed) || len(res3.Written) != 0 {
		t.Errorf("apply on a closed run = %+v, %v; want everything skipped as CLOSED", res3, err)
	}

	// An excluded registration is closed and abandoned, and leaves the queue.
	const excluded = "validation-excluded-co"
	if _, err := NewIngestRepository(pool).Register(ctx, domain.Registration{
		CompanyID: excluded, InitiatingProduct: "agent-manager", CreatedOn: &domain.SourceTime{Time: registered},
	}, nil); err != nil {
		t.Fatalf("register excluded: %v", err)
	}
	var excludedPairing string
	if err := pool.QueryRow(ctx, `
		SELECT op.id::TEXT FROM plg_org_platform op JOIN plg_organization o ON o.id = op.organization_id
		WHERE o.moesif_company_id = $1`, excluded).Scan(&excludedPairing); err != nil {
		t.Fatalf("find excluded pairing: %v", err)
	}
	req := applyReq(excludedPairing)
	disposable, outcome := "DISPOSABLE", domain.OutcomeExcluded
	reasons := []string{"EXCLUDED_DISPOSABLE_EMAIL_ADDRESS"}
	req.Values = append(req.Values[:2],
		domain.ValidationValue{Code: domain.TaskCodeEligibilityOutcome, TextValue: &outcome},
		domain.ValidationValue{Code: domain.TaskCodeEligibilityReason, CheckedCodes: &reasons})
	req.Values[1] = domain.ValidationValue{Code: domain.TaskCodeEmailClassification, TextValue: &disposable}
	req.Abandon = true
	res4, err := repo.Apply(ctx, req)
	if err != nil || !res4.Abandoned {
		t.Fatalf("abandon = %+v, %v", res4, err)
	}
	var stage, reason string
	var closedNow bool
	if err := pool.QueryRow(ctx, `
		SELECT op.lifecycle_stage::TEXT, h.reason,
		       EXISTS (SELECT 1 FROM plg_playbook_run_task WHERE playbook_run_id = $2::UUID AND code = $3 AND is_completed)
		FROM   plg_org_platform op
		JOIN   plg_lifecycle_history h ON h.org_platform_id = op.id AND h.to_stage = 'ABANDONED'
		WHERE  op.id = $1::UUID`, excludedPairing, res4.PlaybookRunID, domain.TaskCodeClose).Scan(&stage, &reason, &closedNow); err != nil {
		t.Fatalf("read abandoned pairing: %v", err)
	}
	if stage != "ABANDONED" || !closedNow || !strings.Contains(reason, "EXCLUDED_DISPOSABLE_EMAIL_ADDRESS") {
		t.Errorf("stage=%s closed=%v reason=%q; want ABANDONED, closed, the reason recorded", stage, closedNow, reason)
	}
	items, _ = repo.Queue(ctx, "AGENT_PLATFORM", registered.Add(-time.Minute), 100)
	if _, ok := queued(items, excluded); ok {
		t.Error("an abandoned pairing is still in the validation queue")
	}
}

// An engineer's write must mark the value MANUAL, or the job would overwrite it.
func TestEngineerTaskWriteMarksTheValueManual(t *testing.T) {
	src, err := os.ReadFile("plg_orgplatform_repo.go")
	if err != nil {
		t.Fatalf("read repository source: %v", err)
	}
	text := string(src)
	start := strings.Index(text, "func (r *orgPlatformRepository) PatchRunTask(")
	end := strings.Index(text[start+1:], "\nfunc ")
	if start < 0 || end < 0 || !strings.Contains(text[start:start+1+end], "value_source  = 'MANUAL'") {
		t.Error("PatchRunTask no longer sets value_source = 'MANUAL'")
	}
}

func applyReq(pairing string) domain.ApplyValidationRequest {
	now := time.Now().UTC()
	status, level, class := "COMPLETED", "HIGH", "CORPORATE"
	minutes := 42.0
	step := "deployed-agent"
	initiate := true
	return domain.ApplyValidationRequest{
		OrgPlatformID: pairing,
		Summary: domain.ActivitySummary{
			WindowStart: now.Add(-time.Hour), WindowEnd: now.Add(71 * time.Hour),
			FirstEventOn: &now, LastEventOn: &now, EventCount: 7,
			ActionCounts: map[string]int{"created-agent": 2, "deployed-agent": 5}, FurthestStep: &step,
		},
		Values: []domain.ValidationValue{
			{Code: domain.TaskCodeInitiate, BoolValue: &initiate},
			{Code: domain.TaskCodeEmailClassification, TextValue: &class},
			{Code: "ONBOARDING_STATUS", TextValue: &status},
			{Code: "INITIAL_SESSION_DURATION_MINUTES", NumberValue: &minutes},
			{Code: "NO_SUCH_TASK", TextValue: &level},
		},
	}
}

func queued(items []domain.ValidationQueueItem, company string) (domain.ValidationQueueItem, bool) {
	for _, it := range items {
		if it.CompanyID == company {
			return it, true
		}
	}
	return domain.ValidationQueueItem{}, false
}

func skipped(res domain.ApplyValidationResult, code, reason string) bool {
	for _, s := range res.Skipped {
		if s.Code == code && s.Reason == reason {
			return true
		}
	}
	return false
}

// seedRegistrationPlaybook authors an active registration playbook with the
// bookends and five of the registration tasks.
func seedRegistrationPlaybook(t *testing.T, product string) {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO plg_playbook (product_id, name, lifecycle_stage, playbook_type, display_order, active)
		VALUES ((SELECT id FROM plg_product WHERE code = $1), 'Registration validation test', 'REGISTRATION', 'PROGRESSIVE', 1, TRUE)
		RETURNING id::TEXT`, product).Scan(&id); err != nil {
		t.Fatalf("seed playbook: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO plg_playbook_task (playbook_id, code, name, sequence_no, value_type, options) VALUES
		($1::UUID, 'INITIATE_PLAYBOOK', 'Initiate', 1, 'BOOLEAN', NULL),
		($1::UUID, 'REGISTERED_EMAIL_CLASSIFICATION', 'Email', 2, 'SINGLE_SELECT',
		    '[{"code":"CORPORATE","label":"Corporate"},{"code":"PERSONAL","label":"Personal"},{"code":"DISPOSABLE","label":"Disposable"}]'),
		($1::UUID, 'INITIAL_SESSION_DURATION_MINUTES', 'Session', 3, 'NUMBER', NULL),
		($1::UUID, 'ONBOARDING_STATUS', 'Onboarding', 4, 'SINGLE_SELECT',
		    '[{"code":"COMPLETED","label":"Completed"},{"code":"DROPPED_OFF","label":"Dropped Off"}]'),
		($1::UUID, 'ELIGIBILITY_ASSESSMENT_OUTCOME', 'Outcome', 5, 'SINGLE_SELECT',
		    '[{"code":"ELIGIBLE","label":"Eligible"},{"code":"MONITOR","label":"Monitor"},{"code":"EXCLUDED","label":"Excluded"}]'),
		($1::UUID, 'ELIGIBILITY_ASSESSMENT_REASON', 'Reason', 6, 'CHECKLIST',
		    '[{"code":"EXCLUDED_DISPOSABLE_EMAIL_ADDRESS","label":"Disposable"},{"code":"ELIGIBLE_CORPORATE_EMAIL_ADDRESS","label":"Corporate"}]'),
		($1::UUID, 'CLOSE_PLAYBOOK', 'Close', 7, 'BOOLEAN', NULL)`, id); err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
}
