package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ValidationRepository serves the plg_registration_validation job: the
// pairings to validate, and one transactional write per pairing.
type ValidationRepository interface {
	Queue(ctx context.Context, productCode string, registeredAfter time.Time, limit int) ([]domain.ValidationQueueItem, error)
	Apply(ctx context.Context, req domain.ApplyValidationRequest) (domain.ApplyValidationResult, error)
}

type validationRepository struct {
	db *pgxpool.Pool
}

// NewValidationRepository returns a ValidationRepository over db.
func NewValidationRepository(db *pgxpool.Pool) ValidationRepository {
	return &validationRepository{db: db}
}

// registrationRunSQL finds the attached registration run of a pairing.
const registrationRunSQL = `
	SELECT r.id::TEXT
	FROM   plg_playbook_run r
	JOIN   plg_playbook pb ON pb.id = r.playbook_id
	WHERE  r.org_platform_id = $1::UUID AND r.detached_on IS NULL
	  AND  pb.lifecycle_stage = 'REGISTRATION' AND pb.playbook_type = 'PROGRESSIVE'
	ORDER  BY r.created_at
	LIMIT  1`

func (r *validationRepository) Queue(ctx context.Context, productCode string, registeredAfter time.Time, limit int) ([]domain.ValidationQueueItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT op.id::TEXT, o.moesif_company_id, o.owner_user_id::TEXT, op.registered_on,
		       (SELECT t.value_text
		        FROM   plg_playbook_run run
		        JOIN   plg_playbook pb ON pb.id = run.playbook_id
		        JOIN   plg_playbook_run_task t ON t.playbook_run_id = run.id
		        WHERE  run.org_platform_id = op.id AND run.detached_on IS NULL
		          AND  pb.lifecycle_stage = 'REGISTRATION' AND pb.playbook_type = 'PROGRESSIVE'
		          AND  t.code = $4
		        ORDER  BY run.created_at
		        LIMIT  1),
		       s.computed_on
		FROM   plg_org_platform op
		JOIN   plg_organization o ON o.id = op.organization_id
		JOIN   plg_product p      ON p.id = op.product_id
		LEFT   JOIN plg_activity_summary s ON s.org_platform_id = op.id
		WHERE  p.code = $1
		  AND  op.lifecycle_stage = 'REGISTRATION'
		  AND  op.registered_on >= $2
		  AND  o.moesif_company_id IS NOT NULL
		ORDER  BY s.computed_on NULLS FIRST, op.registered_on, op.id
		LIMIT  $3`, productCode, registeredAfter, limit, domain.TaskCodeEmailClassification)
	if err != nil {
		return nil, fmt.Errorf("validation queue: %w", err)
	}
	defer rows.Close()

	items := []domain.ValidationQueueItem{}
	for rows.Next() {
		var it domain.ValidationQueueItem
		if err := rows.Scan(&it.OrgPlatformID, &it.CompanyID, &it.OwnerUserID, &it.RegisteredOn,
			&it.EmailClassification, &it.LastValidatedOn); err != nil {
			return nil, fmt.Errorf("scan validation queue: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

type runTaskState struct {
	id        string
	valueType domain.TaskValueType
	options   map[string]bool
	source    *string
	text      *string
}

func (r *validationRepository) Apply(ctx context.Context, req domain.ApplyValidationRequest) (domain.ApplyValidationResult, error) {
	res := domain.ApplyValidationResult{Written: []string{}, Skipped: []domain.SkippedValue{}}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin apply validation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the pairing so a stage change cannot land mid-write.
	var productID, stage string
	err = tx.QueryRow(ctx, `
		SELECT product_id::TEXT, lifecycle_stage::TEXT
		FROM   plg_org_platform WHERE id::TEXT = $1 FOR UPDATE`, req.OrgPlatformID).Scan(&productID, &stage)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, &apierror.NotFoundError{Msg: "pairing not found"}
	}
	if err != nil {
		return res, fmt.Errorf("lock pairing: %w", err)
	}
	if stage != string(domain.StageRegistration) {
		return res, &apierror.ConflictError{Msg: "pairing has left REGISTRATION"}
	}

	runID, attached, err := ensureRegistrationRun(ctx, tx, req.OrgPlatformID, productID)
	if err != nil {
		return res, err
	}
	res.PlaybookRunID, res.Attached = runID, attached

	// Same lock an engineer's task write takes, so the two serialise.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM plg_playbook_run WHERE id = $1::UUID FOR UPDATE`, runID); err != nil {
		return res, fmt.Errorf("lock playbook run: %w", err)
	}

	if err := upsertActivitySummary(ctx, tx, req.OrgPlatformID, req.Summary); err != nil {
		return res, err
	}

	var closed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM plg_playbook_run_task
		               WHERE playbook_run_id = $1::UUID AND code = $2 AND is_completed)`,
		runID, domain.TaskCodeClose).Scan(&closed); err != nil {
		return res, fmt.Errorf("check run closed: %w", err)
	}

	tasks, err := loadRunTaskStates(ctx, tx, runID)
	if err != nil {
		return res, err
	}

	// An exclusion stands only on the classification the run will hold; one an
	// engineer changed since the job read the queue withdraws it, writing nothing.
	if req.Abandon && !closed && !excludingClassification(req, tasks) {
		res.Stale = true
		return res, nil
	}

	for _, v := range req.Values {
		task, ok := tasks[v.Code]
		switch {
		case closed:
			res.Skipped = append(res.Skipped, domain.SkippedValue{Code: v.Code, Reason: domain.SkipClosed})
			continue
		case !ok:
			res.Skipped = append(res.Skipped, domain.SkippedValue{Code: v.Code, Reason: domain.SkipMissing})
			continue
		case isManual(task):
			res.Skipped = append(res.Skipped, domain.SkippedValue{Code: v.Code, Reason: domain.SkipManual})
			continue
		}
		if err := checkValueShape(v, task); err != nil {
			return res, err
		}
		if err := writeValidationValue(ctx, tx, task.id, v); err != nil {
			return res, err
		}
		res.Written = append(res.Written, v.Code)
	}

	// Only when the job's own EXCLUDED outcome landed: an engineer's outcome
	// (MANUAL) or a closed run keeps the pairing where it is.
	closeTask, hasClose := tasks[domain.TaskCodeClose]
	switch {
	case !req.Abandon || closed || !wrote(res.Written, domain.TaskCodeEligibilityOutcome):
	case hasClose && isManual(closeTask):
		// An engineer reopened the run; they decide whether it closes.
		res.Skipped = append(res.Skipped, domain.SkippedValue{Code: domain.TaskCodeClose, Reason: domain.SkipManual})
	default:
		if err := abandonPairing(ctx, tx, req.OrgPlatformID, runID, tasks); err != nil {
			return res, err
		}
		res.Abandoned = true
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit apply validation: %w", err)
	}
	return res, nil
}

// abandonPairing closes the run and moves the pairing out of REGISTRATION, with
// the history row every stage change carries (changed_by NULL: no user).
func abandonPairing(ctx context.Context, tx pgx.Tx, pairingID, runID string, tasks map[string]runTaskState) error {
	if task, ok := tasks[domain.TaskCodeClose]; ok {
		yes := true
		if err := writeValidationValue(ctx, tx, task.id, domain.ValidationValue{Code: domain.TaskCodeClose, BoolValue: &yes}); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE plg_org_platform
		SET    lifecycle_stage = 'ABANDONED', stage_entered_on = NOW(), stage_updated_by = NULL
		WHERE  id::TEXT = $1 AND lifecycle_stage = 'REGISTRATION'`, pairingID)
	if err != nil {
		return fmt.Errorf("abandon pairing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ConflictError{Msg: "pairing has left REGISTRATION"}
	}
	// The reason the run now holds, which may be an engineer's rather than the job's.
	var codes []string
	err = tx.QueryRow(ctx, `
		SELECT value_checked FROM plg_playbook_run_task
		WHERE  playbook_run_id = $1::UUID AND code = $2`, runID, domain.TaskCodeEligibilityReason).Scan(&codes)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read eligibility reason: %w", err)
	}
	reason := "Excluded by registration validation"
	if len(codes) > 0 {
		reason += ": " + strings.Join(codes, ", ")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO plg_lifecycle_history (org_platform_id, from_stage, to_stage, reason, changed_by)
		VALUES ($1::UUID, 'REGISTRATION', 'ABANDONED', $2, NULL)`, pairingID, reason); err != nil {
		return fmt.Errorf("record lifecycle history: %w", err)
	}
	return nil
}

func isManual(t runTaskState) bool {
	return t.source != nil && *t.source == domain.ValueSourceManual
}

// excludingClassification reports whether the classification the run will hold
// after this write is one that excludes: the engineer's when they set it, else
// the job's new value, else the stored one. A run with no such task trusts the job.
func excludingClassification(req domain.ApplyValidationRequest, tasks map[string]runTaskState) bool {
	effective := func() *string {
		task, ok := tasks[domain.TaskCodeEmailClassification]
		if ok && isManual(task) {
			return task.text
		}
		for _, v := range req.Values {
			if v.Code == domain.TaskCodeEmailClassification && v.TextValue != nil {
				return v.TextValue
			}
		}
		if ok {
			return task.text
		}
		return nil
	}()
	_, hasTask := tasks[domain.TaskCodeEmailClassification]
	if effective == nil {
		return !hasTask
	}
	return *effective == "INTERNAL" || *effective == "DISPOSABLE"
}

func wrote(written []string, code string) bool {
	for _, c := range written {
		if c == code {
			return true
		}
	}
	return false
}

// ensureRegistrationRun returns the pairing's registration run, attaching the
// product's active registration playbook (added_by NULL) when there is none.
func ensureRegistrationRun(ctx context.Context, tx pgx.Tx, pairingID, productID string) (string, bool, error) {
	var runID string
	err := tx.QueryRow(ctx, registrationRunSQL, pairingID).Scan(&runID)
	if err == nil {
		return runID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("find registration run: %w", err)
	}

	var playbookID string
	err = tx.QueryRow(ctx, `
		SELECT id::TEXT FROM plg_playbook
		WHERE  product_id = $1::UUID AND lifecycle_stage = 'REGISTRATION'
		  AND  playbook_type = 'PROGRESSIVE' AND active
		ORDER  BY display_order, name
		LIMIT  1`, productID).Scan(&playbookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, &apierror.NotFoundError{Msg: "no active registration playbook for this product"}
	}
	if err != nil {
		return "", false, fmt.Errorf("find registration playbook: %w", err)
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO plg_playbook_run (org_platform_id, playbook_id, added_by)
		VALUES ($1::UUID, $2::UUID, NULL)
		RETURNING id::TEXT`, pairingID, playbookID).Scan(&runID); err != nil {
		return "", false, fmt.Errorf("attach registration playbook: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO plg_playbook_run_task
		    (playbook_run_id, playbook_task_id, code, name, description, sequence_no, value_type, options)
		SELECT $1::UUID, t.id, t.code, t.name, t.description, t.sequence_no, t.value_type, t.options
		FROM   plg_playbook_task t
		WHERE  t.playbook_id = $2::UUID AND t.active
		ON CONFLICT (playbook_run_id, code) DO NOTHING`, runID, playbookID); err != nil {
		return "", false, fmt.Errorf("instantiate run tasks: %w", err)
	}
	return runID, true, nil
}

func upsertActivitySummary(ctx context.Context, tx pgx.Tx, pairingID string, s domain.ActivitySummary) error {
	counts := s.ActionCounts
	if counts == nil {
		counts = map[string]int{}
	}
	raw, err := json.Marshal(counts)
	if err != nil {
		return fmt.Errorf("encode action counts: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO plg_activity_summary
		    (org_platform_id, window_start, window_end, first_event_on, last_event_on,
		     event_count, action_counts, furthest_step, computed_on)
		VALUES ($1::UUID, $2, $3, $4, $5, $6, $7::JSONB, $8, NOW())
		ON CONFLICT (org_platform_id) DO UPDATE
		SET window_start = EXCLUDED.window_start, window_end = EXCLUDED.window_end,
		    first_event_on = EXCLUDED.first_event_on, last_event_on = EXCLUDED.last_event_on,
		    event_count = EXCLUDED.event_count, action_counts = EXCLUDED.action_counts,
		    furthest_step = EXCLUDED.furthest_step, computed_on = NOW()`,
		pairingID, s.WindowStart, s.WindowEnd, s.FirstEventOn, s.LastEventOn,
		s.EventCount, string(raw), s.FurthestStep); err != nil {
		if isCheckViolation(err) {
			return &apierror.ValidationError{Msg: "summary: window, event count or action counts are invalid"}
		}
		return fmt.Errorf("upsert activity summary: %w", err)
	}
	return nil
}

func loadRunTaskStates(ctx context.Context, tx pgx.Tx, runID string) (map[string]runTaskState, error) {
	rows, err := tx.Query(ctx, `
		SELECT code, id::TEXT, value_type::TEXT, options, value_source::TEXT, value_text
		FROM   plg_playbook_run_task WHERE playbook_run_id = $1::UUID`, runID)
	if err != nil {
		return nil, fmt.Errorf("load run tasks: %w", err)
	}
	defer rows.Close()

	out := map[string]runTaskState{}
	for rows.Next() {
		var (
			code    string
			t       runTaskState
			rawOpts []byte
		)
		if err := rows.Scan(&code, &t.id, &t.valueType, &rawOpts, &t.source, &t.text); err != nil {
			return nil, fmt.Errorf("scan run task: %w", err)
		}
		opts, err := scanOptions(rawOpts)
		if err != nil {
			return nil, err
		}
		t.options = make(map[string]bool, len(opts))
		for _, o := range opts {
			t.options[o.Code] = true
		}
		out[code] = t
	}
	return out, rows.Err()
}

// checkValueShape refuses a value the task cannot hold. No value at all clears it.
func checkValueShape(v domain.ValidationValue, t runTaskState) error {
	bad := func(want string) error {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s takes %s", v.Code, want)}
	}
	sent := 0
	for _, set := range []bool{v.BoolValue != nil, v.NumberValue != nil, v.TextValue != nil, v.CheckedCodes != nil} {
		if set {
			sent++
		}
	}
	if sent == 0 {
		return nil
	}
	if sent > 1 {
		return &apierror.ValidationError{Msg: v.Code + ": send one value field"}
	}
	switch t.valueType {
	case domain.ValueBoolean:
		if v.BoolValue == nil {
			return bad("boolValue")
		}
	case domain.ValueNumber:
		if v.NumberValue == nil {
			return bad("numberValue")
		}
	case domain.ValueString:
		if v.TextValue == nil {
			return bad("textValue")
		}
	case domain.ValueSingleSelect:
		if v.TextValue == nil {
			return bad("textValue")
		}
		if !t.options[*v.TextValue] {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s has no option %q", v.Code, *v.TextValue)}
		}
	case domain.ValueChecklist:
		if v.CheckedCodes == nil {
			return bad("checkedCodes")
		}
		for _, c := range *v.CheckedCodes {
			if !t.options[c] {
				return &apierror.ValidationError{Msg: fmt.Sprintf("%s has no option %q", v.Code, c)}
			}
		}
	}
	return nil
}

func writeValidationValue(ctx context.Context, tx pgx.Tx, taskID string, v domain.ValidationValue) error {
	var checked *[]string
	if v.CheckedCodes != nil && len(*v.CheckedCodes) > 0 {
		checked = v.CheckedCodes
	}
	// The MANUAL guard repeats inside the statement; the run lock makes it a
	// formality, but it keeps the rule in one place a reader can see.
	tag, err := tx.Exec(ctx, `
		UPDATE plg_playbook_run_task
		SET    value_bool = $2, value_number = $3, value_text = $4, value_checked = $5::TEXT[],
		       value_source = 'VALIDATION'
		WHERE  id = $1::UUID AND value_source IS DISTINCT FROM 'MANUAL'
		  AND  EXISTS (SELECT 1 FROM plg_playbook_run r
		               WHERE r.id = plg_playbook_run_task.playbook_run_id
		                 AND r.detached_on IS NULL)`,
		taskID, v.BoolValue, v.NumberValue, v.TextValue, checked)
	if err != nil {
		if isConstraintViolation(err) {
			return &apierror.ValidationError{Msg: v.Code + ": value does not match the task's type"}
		}
		return fmt.Errorf("write %s: %w", v.Code, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("write %s: task changed mid-transaction", v.Code)
	}
	// Completion follows the value; the job is no user, so completed_by stays NULL.
	if _, err := tx.Exec(ctx, `
		UPDATE plg_playbook_run_task
		SET    completed_on = CASE WHEN is_completed THEN COALESCE(completed_on, NOW()) ELSE NULL END,
		       completed_by = CASE WHEN is_completed THEN completed_by ELSE NULL END
		WHERE  id = $1::UUID
		  AND  EXISTS (SELECT 1 FROM plg_playbook_run r
		               WHERE r.id = plg_playbook_run_task.playbook_run_id
		                 AND r.detached_on IS NULL)`, taskID); err != nil {
		return fmt.Errorf("stamp completion %s: %w", v.Code, err)
	}
	return nil
}
