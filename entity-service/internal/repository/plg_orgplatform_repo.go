package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OrgPlatformRepository owns the organisation+platform pairing — the unit of
// work. Its lifecycle stage, the playbooks running on it, their typed task
// values, and its comment trail.
type OrgPlatformRepository interface {
	Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error)
	Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) (domain.PatchPairingResult, error)
	Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actor string) (domain.AcknowledgeResult, error)

	AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actor string) error
	DetachRun(ctx context.Context, runID string) (res domain.WriteResult, orgID, productCode string, err error)
	PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actor string) (orgID, productCode string, err error)
	RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error)

	CreateNote(ctx context.Context, req domain.CreateNoteRequest, actor string) error
	UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actor string) (res domain.WriteResult, orgID, productCode string, err error)
	SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error)
	LocatePairing(ctx context.Context, orgPlatformID string) (orgID, productCode string, err error)
}

type orgPlatformRepository struct {
	db        *pgxpool.Pool
	playbooks PlaybookRepository
}

// NewOrgPlatformRepository builds an OrgPlatformRepository. It composes the
// playbook repository rather than duplicating its queries, so the product tab's
// "playbooks you can add here" list is the same code the manager uses.
func NewOrgPlatformRepository(db *pgxpool.Pool, playbooks PlaybookRepository) OrgPlatformRepository {
	return &orgPlatformRepository{db: db, playbooks: playbooks}
}

// resolve turns (organisation id, product code-or-id) into the pairing id.
func (r *orgPlatformRepository) resolve(ctx context.Context, orgID, productCode string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT op.id::TEXT
		FROM   plg_org_platform op
		JOIN   plg_product p ON p.id = op.product_id
		WHERE  op.organization_id::TEXT = $1 AND (p.code = $2 OR p.id::TEXT = $2)`,
		orgID, productCode).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &apierror.NotFoundError{Msg: "this organisation is not registered for that platform"}
	}
	if err != nil {
		return "", fmt.Errorf("resolve pairing: %w", err)
	}
	return id, nil
}

// LocatePairing turns a pairing id back into the (organisation, product code)
// pair the read path is addressed by. Write endpoints that only learn the
// pairing id use it to reload the product tab.
func (r *orgPlatformRepository) LocatePairing(ctx context.Context, orgPlatformID string) (string, string, error) {
	var orgID, code string
	err := r.db.QueryRow(ctx, `
		SELECT op.organization_id::TEXT, p.code
		FROM   plg_org_platform op
		JOIN   plg_product p ON p.id = op.product_id
		WHERE  op.id::TEXT = $1`, orgPlatformID).Scan(&orgID, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", &apierror.NotFoundError{Msg: "pairing not found"}
	}
	if err != nil {
		return "", "", fmt.Errorf("locate pairing: %w", err)
	}
	return orgID, code, nil
}

// ---------------------------------------------------------------------------
// Read — the product tab
// ---------------------------------------------------------------------------

func (r *orgPlatformRepository) Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	const q = `
		SELECT v.org_platform_id, v.organization_id, v.organization_name,
		       v.product_id, v.product_code, v.product_name,
		       v.lifecycle_stage::TEXT, v.lifecycle_stage_name, v.stage_entered_on,
		       v.health_state::TEXT, v.health_entered_on,
		       v.health_updated_by::TEXT, hu.email, hu.display_name,
		       v.applicable_playbook_types,
		       v.stage_carries_playbooks,
		       v.registered_on, v.registered_email,
		       v.subscription_tier::TEXT, v.subscription_entered_on,
		       v.subscription_updated_by::TEXT, su.email, su.display_name,
		       v.trial_end_date, v.trial_extended_date, v.current_period_end_date,
		       v.acknowledged_on, v.acknowledged_by::TEXT, cu.email, cu.display_name,
		       v.is_new,
		       v.run_total::INT, v.run_active::INT, v.run_closed::INT,
		       v.task_total::INT, v.task_completed::INT
		FROM   plg_org_platform_v v
		LEFT   JOIN plg_user_v cu ON cu.id = v.acknowledged_by
		LEFT   JOIN plg_user_v hu ON hu.id = v.health_updated_by
		LEFT   JOIN plg_user_v su ON su.id = v.subscription_updated_by
		JOIN   plg_org_platform op ON op.id = v.org_platform_id
		JOIN   plg_product p ON p.id = op.product_id
		WHERE  v.organization_id::TEXT = $1 AND (p.code = $2 OR p.id::TEXT = $2)`

	var (
		d                          domain.ProductDetail
		prodID, prodCode, prodName string
		tier                       *string
		ackID, ackEmail, ackName   *string
		hID, hEmail, hName         *string
		sID, sEmail, sName         *string
	)
	err := r.db.QueryRow(ctx, q, orgID, productCode).Scan(
		&d.OrgPlatformID, &d.OrganizationID, &d.OrganizationName,
		&prodID, &prodCode, &prodName,
		&d.LifecycleStage, &d.StageName, &d.StageEnteredOn,
		&d.HealthState, &d.HealthEnteredOn, &hID, &hEmail, &hName,
		&d.ApplicablePlaybookTypes,
		&d.CarriesPlaybooks,
		&d.RegisteredOn, &d.RegisteredEmail,
		&tier, &d.SubscriptionEnteredOn, &sID, &sEmail, &sName,
		&d.TrialEndDate, &d.TrialExtendedDate, &d.CurrentPeriodEndDate,
		&d.AcknowledgedOn, &ackID, &ackEmail, &ackName,
		&d.IsNew,
		&d.Progress.RunTotal, &d.Progress.RunActive, &d.Progress.RunClosed,
		&d.Progress.TaskTotal, &d.Progress.TaskCompleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "this organisation is not registered for that platform"}
	}
	if err != nil {
		return nil, fmt.Errorf("load pairing: %w", err)
	}
	d.Product = productRef(prodID, prodCode, prodName)
	d.HealthUpdatedBy = userRef(hID, hEmail, hName)
	d.SubscriptionUpdatedBy = userRef(sID, sEmail, sName)
	d.AcknowledgedBy = userRef(ackID, ackEmail, ackName)
	if tier != nil {
		t := domain.SubscriptionTier(*tier)
		d.SubscriptionTier = &t
	}

	if d.Runs, err = r.loadRuns(ctx, d.OrgPlatformID); err != nil {
		return nil, err
	}
	if d.Notes, err = r.loadNotes(ctx, d.OrgPlatformID); err != nil {
		return nil, err
	}
	if d.LifecycleHistory, err = r.loadHistory(ctx, d.OrgPlatformID); err != nil {
		return nil, err
	}

	// What can be added here: the active playbooks for this product, at this
	// pairing's stage, OF THE KIND ITS HEALTH CALLS FOR, minus the ones already
	// running.
	//
	// A healthy pairing is offered the work that moves
	// it on and the work that keeps it steady; an at-risk one is offered
	// recovery work and nothing else. The list never mixes the two situations,
	// so an engineer opening the menu sees only plays that make sense for the
	// state the pairing is actually in.
	available, err := r.playbooks.ListForStage(ctx, prodID, d.LifecycleStage,
		domain.ApplicablePlaybookTypes(d.HealthState))
	if err != nil {
		return nil, err
	}
	running := make(map[string]bool, len(d.Runs))
	for _, run := range d.Runs {
		running[run.PlaybookID] = true
	}
	d.AvailablePlaybooks = []domain.Playbook{}
	for _, pb := range available {
		if !running[pb.ID] {
			d.AvailablePlaybooks = append(d.AvailablePlaybooks, pb)
		}
	}
	return &d, nil
}

// loadRuns returns every playbook running on a pairing, each with its tasks.
// Run status comes from plg_playbook_run_v, which derives it from the bookends.
func (r *orgPlatformRepository) loadRuns(ctx context.Context, pairingID string) ([]domain.PlaybookRun, error) {
	const q = `
		SELECT v.playbook_run_id, v.org_platform_id, v.playbook_id, v.playbook_name,
		       v.playbook_description, v.playbook_stage::TEXT, v.playbook_type::TEXT,
		       v.run_status, v.task_total::INT, v.task_completed::INT, v.next_task_name,
		       v.added_by::TEXT, cu.email, cu.display_name, v.created_at
		FROM   plg_playbook_run_v v
		LEFT   JOIN plg_user_v cu ON cu.id = v.added_by
		WHERE  v.org_platform_id::TEXT = $1
		ORDER  BY v.created_at`

	rows, err := r.db.Query(ctx, q, pairingID)
	if err != nil {
		return nil, fmt.Errorf("load playbook runs: %w", err)
	}

	runs := []domain.PlaybookRun{}
	ids := []string{}
	for rows.Next() {
		var (
			run                            domain.PlaybookRun
			addedID, addedEmail, addedName *string
		)
		if err := rows.Scan(&run.ID, &run.OrgPlatformID, &run.PlaybookID, &run.PlaybookName,
			&run.Description, &run.PlaybookStage, &run.PlaybookType,
			&run.RunStatus, &run.TaskTotal, &run.TaskCompleted, &run.NextTaskName,
			&addedID, &addedEmail, &addedName, &run.CreatedOn); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan playbook run: %w", err)
		}
		run.AddedBy = userRef(addedID, addedEmail, addedName)
		run.Tasks = []domain.RunTask{}
		runs = append(runs, run)
		ids = append(ids, run.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playbook runs: %w", err)
	}
	if len(runs) == 0 {
		return runs, nil
	}

	tasks, err := r.loadRunTasks(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		if t, ok := tasks[runs[i].ID]; ok {
			runs[i].Tasks = t
		}
	}
	return runs, nil
}

func (r *orgPlatformRepository) loadRunTasks(ctx context.Context, runIDs []string) (map[string][]domain.RunTask, error) {
	const q = `
		SELECT t.playbook_run_id::TEXT, t.id::TEXT, t.playbook_task_id::TEXT,
		       t.code, t.name, t.description, t.sequence_no, t.value_type::TEXT,
		       t.value_bool, t.value_number, t.value_text,
		       t.options, COALESCE(t.value_checked, '{}'), t.is_completed,
		       t.completed_on, t.completed_by::TEXT, cu.email, cu.display_name
		FROM   plg_playbook_run_task t
		LEFT   JOIN plg_user_v cu ON cu.id = t.completed_by
		WHERE  t.playbook_run_id::TEXT = ANY($1::TEXT[])
		ORDER  BY t.sequence_no`

	rows, err := r.db.Query(ctx, q, runIDs)
	if err != nil {
		return nil, fmt.Errorf("load run tasks: %w", err)
	}
	defer rows.Close()

	out := map[string][]domain.RunTask{}
	for rows.Next() {
		var (
			runID                       string
			t                           domain.RunTask
			rawOpts                     []byte
			doneID, doneEmail, doneName *string
		)
		if err := rows.Scan(&runID, &t.ID, &t.PlaybookTaskID, &t.Code, &t.Name, &t.Description,
			&t.SequenceNo, &t.ValueType, &t.BoolValue, &t.NumberValue, &t.TextValue,
			&rawOpts, &t.CheckedCodes, &t.IsCompleted, &t.CompletedOn,
			&doneID, &doneEmail, &doneName); err != nil {
			return nil, fmt.Errorf("scan run task: %w", err)
		}
		opts, err := scanOptions(rawOpts)
		if err != nil {
			return nil, err
		}
		t.Options = opts
		t.PlaybookRunID = runID
		t.CompletedBy = userRef(doneID, doneEmail, doneName)
		t.IsBookend = domain.IsBookendTask(t.Code)
		out[runID] = append(out[runID], t)
	}
	return out, rows.Err()
}

func (r *orgPlatformRepository) loadNotes(ctx context.Context, pairingID string) ([]domain.Note, error) {
	const q = `
		SELECT n.id::TEXT, n.body, n.author::TEXT, cu.email, cu.display_name, n.created_on,
		       n.updated_on, n.updated_by::TEXT, eu.email, eu.display_name
		FROM   plg_note n
		LEFT   JOIN plg_user_v cu ON cu.id = n.author
		LEFT   JOIN plg_user_v eu ON eu.id = n.updated_by
		WHERE  n.org_platform_id::TEXT = $1
		ORDER  BY n.created_on DESC`

	rows, err := r.db.Query(ctx, q, pairingID)
	if err != nil {
		return nil, fmt.Errorf("load notes: %w", err)
	}
	defer rows.Close()

	notes := []domain.Note{}
	for rows.Next() {
		var (
			n                           domain.Note
			authID, authEmail, authName *string
			editID, editEmail, editName *string
		)
		if err := rows.Scan(&n.ID, &n.Body, &authID, &authEmail, &authName, &n.CreatedOn,
			&n.UpdatedOn, &editID, &editEmail, &editName); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.Author = userRef(authID, authEmail, authName)
		n.UpdatedBy = userRef(editID, editEmail, editName)
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func (r *orgPlatformRepository) loadHistory(ctx context.Context, pairingID string) ([]domain.LifecycleEntry, error) {
	// LEFT JOIN on the stage, not JOIN. to_stage is nullable now: a row can
	// record a health change and nothing else, and an inner join would silently
	// drop exactly those rows — the timeline would show stage moves only and
	// look complete while being half the story.
	const q = `
		SELECT h.id::TEXT, h.from_stage::TEXT, h.to_stage::TEXT, ls.name,
		       h.from_health::TEXT, h.to_health::TEXT,
		       h.from_subscription::TEXT, h.to_subscription::TEXT, h.reason,
		       h.changed_by::TEXT, cu.email, cu.display_name, h.changed_on
		FROM   plg_lifecycle_history h
		LEFT   JOIN plg_lifecycle_stage ls ON ls.stage = h.to_stage
		LEFT   JOIN plg_user_v cu ON cu.id = h.changed_by
		WHERE  h.org_platform_id::TEXT = $1
		ORDER  BY h.changed_on DESC`

	rows, err := r.db.Query(ctx, q, pairingID)
	if err != nil {
		return nil, fmt.Errorf("load lifecycle history: %w", err)
	}
	defer rows.Close()

	entries := []domain.LifecycleEntry{}
	for rows.Next() {
		var (
			e                     domain.LifecycleEntry
			from, to              *string
			fromHealth, toHealth  *string
			fromSub, toSub        *string
			byID, byEmail, byName *string
		)
		if err := rows.Scan(&e.ID, &from, &to, &e.ToName,
			&fromHealth, &toHealth, &fromSub, &toSub, &e.Reason,
			&byID, &byEmail, &byName, &e.ChangedOn); err != nil {
			return nil, fmt.Errorf("scan lifecycle entry: %w", err)
		}
		if from != nil {
			s := domain.LifecycleStage(*from)
			e.FromStage = &s
		}
		if to != nil {
			s := domain.LifecycleStage(*to)
			e.ToStage = &s
		}
		if fromHealth != nil {
			h := domain.HealthState(*fromHealth)
			e.FromHealth = &h
		}
		if toHealth != nil {
			h := domain.HealthState(*toHealth)
			e.ToHealth = &h
		}
		if fromSub != nil {
			t := domain.SubscriptionTier(*fromSub)
			e.FromSubscription = &t
		}
		if toSub != nil {
			t := domain.SubscriptionTier(*toSub)
			e.ToSubscription = &t
		}
		e.ChangedBy = userRef(byID, byEmail, byName)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ---------------------------------------------------------------------------
// Write
// ---------------------------------------------------------------------------

// Patch updates the pairing's stage, use case, tier or trial date.
//
// GUARDED WRITE (contract W2). The stage the caller believed was current goes
// into the WHERE clause: the update applies only while that is still true, and
// zero rows means someone moved it first. A lock cannot span an HTTP boundary,
// so the decision travels in the statement instead.
//
// A stage change ALWAYS writes a history row, and that rule lives here rather
// than in the BFF. It is an integrity invariant of these two tables — the
// history must not be able to disagree with the stage — so it belongs with the
// data. What the BFF still decides is whether the stage changes at all.
//
// A stage change is not validated against the path table: those paths say where
// playbooks apply, not what moves are legal. An owner records whatever stage is
// true — which is the only way anything reaches Disqualified, At Risk or
// Abandoned, since no path leads to them.
func (r *orgPlatformRepository) Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) (domain.PatchPairingResult, error) {
	var res domain.PatchPairingResult

	pairingID, err := r.resolve(ctx, req.OrganizationID, req.ProductCode)
	if err != nil {
		return res, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin patch pairing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if req.LifecycleStage != nil {
		// The precondition. Absent one, the caller is saying "set it regardless"
		// — a blind write, which stays available; the BFF sends ExpectedStage
		// when it read a stage and decided from it.
		guard := ""
		args := []any{pairingID, string(*req.LifecycleStage), uuidArg(actor)}
		if req.ExpectedStage != nil {
			guard = ` AND lifecycle_stage = $4::plg_lifecycle_stage_enum`
			args = append(args, string(*req.ExpectedStage))
		}

		var from string
		err = tx.QueryRow(ctx, `
			UPDATE plg_org_platform
			SET    lifecycle_stage  = $2::plg_lifecycle_stage_enum,
			       stage_entered_on = NOW(),
			       stage_updated_by = $3::UUID
			WHERE  id::TEXT = $1`+guard+`
			RETURNING (SELECT lifecycle_stage::TEXT FROM plg_org_platform WHERE id::TEXT = $1)`,
			args...).Scan(&from)

		if errors.Is(err, pgx.ErrNoRows) {
			// Precondition failed. Report the stage that is actually current, so
			// the BFF can tell the caller what happened rather than only that it
			// lost.
			var current string
			if e := tx.QueryRow(ctx,
				`SELECT lifecycle_stage::TEXT FROM plg_org_platform WHERE id::TEXT = $1`,
				pairingID).Scan(&current); e != nil {
				if errors.Is(e, pgx.ErrNoRows) {
					return res, &apierror.NotFoundError{Msg: "this organisation is not registered for that platform"}
				}
				return res, fmt.Errorf("read current stage: %w", e)
			}
			res.CurrentStage = domain.LifecycleStage(current)
			return res, nil // RowsAffected 0 — the BFF turns that into a 409
		}
		if err != nil {
			return res, actorWrite(err, actor, "update lifecycle stage")
		}

		// The invariant: a stage change is recorded. Same transaction, so the
		// history cannot be missing for a change that happened.
		if _, err := tx.Exec(ctx, `
			INSERT INTO plg_lifecycle_history (org_platform_id, from_stage, to_stage, reason, changed_by)
			VALUES ($1::UUID, $2::plg_lifecycle_stage_enum, $3::plg_lifecycle_stage_enum, $4, $5::UUID)`,
			pairingID, from, string(*req.LifecycleStage), req.Reason, uuidArg(actor)); err != nil {
			return res, actorWrite(err, actor, "record lifecycle history")
		}
		res.CurrentStage = *req.LifecycleStage
		res.RowsAffected = 1
	}

	// The second axis. Independent of the stage — a request may move either,
	// both or neither — but written in the SAME transaction, because an
	// engineer who moves a pairing forward and marks it healthy in one action
	// performed one act of judgement, and a crash between the two halves would
	// leave a record that never happened.
	//
	// Guarded on the value actually changing, so re-sending the current health
	// does not litter the history with rows saying nothing moved.
	if req.HealthState != nil {
		var fromHealth string
		err := tx.QueryRow(ctx, `
			UPDATE plg_org_platform
			SET    health_state      = $2::plg_health_enum,
			       health_entered_on = NOW(),
			       health_updated_by = $3::UUID
			WHERE  id::TEXT = $1 AND health_state <> $2::plg_health_enum
			RETURNING (SELECT health_state::TEXT FROM plg_org_platform WHERE id::TEXT = $1)`,
			pairingID, string(*req.HealthState), uuidArg(actor)).Scan(&fromHealth)

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Already in that state. Not an error and not a change: the caller
			// asked for something that is already true.
		case err != nil:
			return res, actorWrite(err, actor, "update health")
		default:
			if _, err := tx.Exec(ctx, `
				INSERT INTO plg_lifecycle_history (org_platform_id, from_health, to_health, reason, changed_by)
				VALUES ($1::UUID, $2::plg_health_enum, $3::plg_health_enum, $4, $5::UUID)`,
				pairingID, fromHealth, string(*req.HealthState), req.Reason,
				uuidArg(actor)); err != nil {
				return res, actorWrite(err, actor, "record health history")
			}
			res.RowsAffected = 1
		}
	}

	// The third axis, written the same way health is: guarded on the value
	// actually changing, so re-sending the current tier does not litter the
	// history with rows saying nothing moved.
	//
	// A tier change is recorded because "who said this customer went PayG, and
	// when, and why" is a question that gets asked and has nowhere else to be
	// answered from. The reason is required — enforced in the BFF, not here.
	if req.SubscriptionTier != nil {
		var fromTier *string
		err := tx.QueryRow(ctx, `
			UPDATE plg_org_platform
			SET    subscription_tier       = $2::plg_subscription_tier_enum,
			       subscription_entered_on = NOW(),
			       subscription_updated_by = $3::UUID
			WHERE  id::TEXT = $1
			  AND  subscription_tier IS DISTINCT FROM $2::plg_subscription_tier_enum
			RETURNING (SELECT subscription_tier::TEXT FROM plg_org_platform WHERE id::TEXT = $1)`,
			pairingID, string(*req.SubscriptionTier), uuidArg(actor)).Scan(&fromTier)

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Already on that tier. Not an error and not a change.
		case err != nil:
			return res, actorWrite(err, actor, "update subscription tier")
		default:
			if _, err := tx.Exec(ctx, `
				INSERT INTO plg_lifecycle_history
				    (org_platform_id, from_subscription, to_subscription, reason, changed_by)
				VALUES ($1::UUID, $2::plg_subscription_tier_enum, $3::plg_subscription_tier_enum, $4, $5::UUID)`,
				pairingID, fromTier, string(*req.SubscriptionTier), req.Reason,
				uuidArg(actor)); err != nil {
				return res, actorWrite(err, actor, "record subscription history")
			}
			res.RowsAffected = 1
		}
	}

	// The two trial dates take identical treatment — set a day, or clear it —
	// so they share one writer rather than two near-copies that could drift.
	// The column name is chosen here from a closed set, never from input.
	writeDate := func(column string, clear bool, value *string, what string) error {
		if !clear && value == nil {
			return nil
		}
		var date *string
		if !clear {
			date = value
		}
		if _, err := tx.Exec(ctx, `
			UPDATE plg_org_platform
			SET    `+column+` = CAST(NULLIF($2::TEXT, '') AS DATE)
			WHERE  id::TEXT = $1`, pairingID, date); err != nil {
			return fmt.Errorf("update %s: %w", what, err)
		}
		res.RowsAffected = 1
		return nil
	}
	if err := writeDate("trial_end_date", req.ClearTrialEndDate,
		req.TrialEndDate, "trial end date"); err != nil {
		return res, err
	}
	if err := writeDate("trial_extended_date", req.ClearTrialExtendedDate,
		req.TrialExtendedDate, "trial extended date"); err != nil {
		return res, err
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit patch pairing: %w", err)
	}
	return res, nil
}

// Acknowledge claims a registration and assigns the organisation's CS owner.
//
// GUARDED WRITE (contract W1). A lock cannot span an HTTP boundary, so the
// decision travels in the statement: the UPDATE only matches while
// acknowledged_by IS NULL, and the row count says whether it still did.
//
// This layer does not know that zero rows means 409. The BFF decides — see
// plg-docs/ENTITY-SERVICE-CONTRACT.md.
func (r *orgPlatformRepository) Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actor string) (domain.AcknowledgeResult, error) {
	// Defaults to the acting engineer: acknowledging a registration claims it,
	// unless the caller names someone else. Both are ids.
	owner := actor
	if req.OwnerID != nil && *req.OwnerID != "" {
		owner = *req.OwnerID
	}

	var res domain.AcknowledgeResult

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin acknowledge: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The guard and the write are one statement. RETURNING tells us both that it
	// applied and which organisation to assign below, so no separate read is
	// needed for that.
	var orgID, productCode string
	err = tx.QueryRow(ctx, `
		UPDATE plg_org_platform op
		SET    acknowledged_on = NOW(), acknowledged_by = $2::UUID
		FROM   plg_product pr
		WHERE  op.id::TEXT = $1
		  AND  op.acknowledged_by IS NULL
		  AND  pr.id = op.product_id
		RETURNING op.organization_id::TEXT, pr.code`,
		req.OrgPlatformID, uuidArg(owner)).Scan(&orgID, &productCode)

	if errors.Is(err, pgx.ErrNoRows) {
		// Either the pairing does not exist or it is already acknowledged. The
		// two are distinguished here rather than left to the caller, because
		// only this layer can tell them apart without a second round trip.
		var exists bool
		if e := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM plg_org_platform WHERE id::TEXT = $1)`,
			req.OrgPlatformID).Scan(&exists); e != nil {
			return res, fmt.Errorf("check pairing: %w", e)
		}
		if !exists {
			return res, &apierror.NotFoundError{Msg: "registration not found"}
		}
		return res, nil // RowsAffected stays 0 — the BFF turns that into a 409
	}
	if err != nil {
		if isConstraintViolation(err) {
			return res, &apierror.ValidationError{Msg: "unknown CS user: " + owner}
		}
		return res, fmt.Errorf("acknowledge: %w", err)
	}

	// Assign the owner only if the organisation has none — acknowledging a second
	// platform should not quietly reassign a customer someone else already owns.
	// Guarded the same way: the WHERE clause carries the condition.
	if _, err := tx.Exec(ctx, `
		UPDATE plg_organization
		SET    plg_cs_owner = $2::UUID
		WHERE  id::TEXT = $1 AND plg_cs_owner IS NULL`, orgID, uuidArg(owner)); err != nil {
		return res, fmt.Errorf("assign owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit acknowledge: %w", err)
	}
	return domain.AcknowledgeResult{RowsAffected: 1, OrganizationID: orgID, ProductCode: productCode}, nil
}

// AttachPlaybook adds a run and copies the template's active tasks onto it,
// value types included. Idempotent: re-attaching is a no-op.
func (r *orgPlatformRepository) AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actor string) error {
	pairingID, err := r.resolve(ctx, req.OrganizationID, req.ProductCode)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin attach playbook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// GUARDED WRITE (contract W3). The INSERT is its own source for `active`,
	// rather than reading it and deciding first: a playbook deactivated between
	// those two steps cannot slip through, because there are no two steps.
	//
	// ON CONFLICT DO NOTHING makes re-attaching a no-op.
	var runID string
	err = tx.QueryRow(ctx, `
		INSERT INTO plg_playbook_run (org_platform_id, playbook_id, added_by)
		SELECT $1::UUID, pb.id, $3::UUID
		FROM   plg_playbook pb
		WHERE  pb.id::TEXT = $2 AND pb.active
		ON CONFLICT (org_platform_id, playbook_id) DO NOTHING
		RETURNING id::TEXT`, pairingID, req.PlaybookID, uuidArg(actor)).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either it is already attached, or the playbook is absent/inactive. The
		// first is success (attaching is idempotent); the second is a 400. Only a
		// second look can tell them apart, and it is cheap.
		var attached bool
		if e := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM plg_playbook_run
			               WHERE org_platform_id::TEXT = $1 AND playbook_id::TEXT = $2)`,
			pairingID, req.PlaybookID).Scan(&attached); e != nil {
			return fmt.Errorf("check existing run: %w", e)
		}
		if attached {
			return nil // already attached — attaching is idempotent
		}
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM plg_playbook WHERE id::TEXT = $1)`,
			req.PlaybookID).Scan(&exists); e != nil {
			return fmt.Errorf("check playbook: %w", e)
		}
		if !exists {
			return &apierror.NotFoundError{Msg: "playbook not found"}
		}
		return &apierror.ValidationError{Msg: "playbook is inactive"}
	}
	if err != nil {
		// The trg_plg_playbook_run_product trigger refuses a playbook belonging to
		// another product, raising a check violation. An unrecognised caller
		// raises a foreign-key violation from added_by instead — same statement,
		// different cause, so they must not share a message.
		if isCheckViolation(err) {
			return &apierror.ValidationError{Msg: "that playbook belongs to a different product"}
		}
		return actorWrite(err, actor, "attach playbook")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO plg_playbook_run_task
		    (playbook_run_id, playbook_task_id, code, name, description, sequence_no, value_type, options)
		SELECT $1::UUID, t.id, t.code, t.name, t.description, t.sequence_no, t.value_type, t.options
		FROM   plg_playbook_task t
		WHERE  t.playbook_id::TEXT = $2 AND t.active
		ON CONFLICT (playbook_run_id, code) DO NOTHING`, runID, req.PlaybookID); err != nil {
		return fmt.Errorf("instantiate run tasks: %w", err)
	}

	return tx.Commit(ctx)
}

// DetachRun removes a run and its task instances.
//
// GUARDED WRITE (contract W4). A closed run cannot be detached — that would
// erase a recorded outcome. The refusal is in the DELETE's WHERE clause rather
// than a read before it, so a run closed in between is still protected.
func (r *orgPlatformRepository) DetachRun(ctx context.Context, runID string) (domain.WriteResult, string, string, error) {
	var res domain.WriteResult

	// Locate first: the caller's response is the reloaded pairing, so the ids are
	// needed whether or not the delete applies.
	var pairingID string
	err := r.db.QueryRow(ctx,
		`SELECT org_platform_id::TEXT FROM plg_playbook_run WHERE id::TEXT = $1`,
		runID).Scan(&pairingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, "", "", &apierror.NotFoundError{Msg: "playbook run not found"}
	}
	if err != nil {
		return res, "", "", fmt.Errorf("read playbook run: %w", err)
	}

	orgID, code, err := r.LocatePairing(ctx, pairingID)
	if err != nil {
		return res, "", "", err
	}

	// The guard: no completed CLOSE_PLAYBOOK task on this run. Expressed against
	// the task table rather than the view's run_status, because a WHERE clause
	// cannot reference a derived column of the row it is deleting.
	tag, err := r.db.Exec(ctx, `
		DELETE FROM plg_playbook_run r
		WHERE  r.id::TEXT = $1
		  AND  NOT EXISTS (SELECT 1 FROM plg_playbook_run_task t
		                   WHERE t.playbook_run_id = r.id
		                     AND t.code = $2 AND t.is_completed)`,
		runID, domain.TaskCodeClose)
	if err != nil {
		return res, "", "", fmt.Errorf("detach playbook run: %w", err)
	}
	res.RowsAffected = int(tag.RowsAffected())
	return res, orgID, code, nil
}

// RunTaskShape is what the service layer needs to know before writing a value:
// RunTaskShape reports a task instance's declared type and — for a checklist —
// which reasons it offers, so a ticked code that is not one of them can be
// refused by name rather than by a constraint violation.
func (r *orgPlatformRepository) RunTaskShape(ctx context.Context, taskID string) (domain.RunTaskShape, error) {
	var shape domain.RunTaskShape
	err := r.db.QueryRow(ctx, `
		SELECT t.value_type::TEXT,
		       COALESCE(ARRAY(SELECT o ->> 'code'
		                      FROM   jsonb_array_elements(t.options) o), '{}')
		FROM   plg_playbook_run_task t
		WHERE  t.id::TEXT = $1`, taskID).Scan(&shape.ValueType, &shape.OptionCodes)
	if errors.Is(err, pgx.ErrNoRows) {
		return shape, &apierror.NotFoundError{Msg: "task not found"}
	}
	if err != nil {
		return shape, fmt.Errorf("read task shape: %w", err)
	}
	return shape, nil
}

// PatchRunTask records a task's value.
//
// is_completed is a generated column, so it is never written here — it follows
// the value automatically. completed_on and completed_by are set to match, and
// cleared when the value is.
func (r *orgPlatformRepository) PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actor string) (string, string, error) {
	var pairingID string
	err := r.db.QueryRow(ctx, `
		SELECT run.org_platform_id::TEXT
		FROM   plg_playbook_run_task t
		JOIN   plg_playbook_run run ON run.id = t.playbook_run_id
		WHERE  t.id::TEXT = $1`, req.ID).Scan(&pairingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", &apierror.NotFoundError{Msg: "task not found"}
	}
	if err != nil {
		return "", "", fmt.Errorf("locate task: %w", err)
	}

	// value_checked differs from the other three: it is replaced outright rather
	// than coalesced, because the caller sends the whole set of ticked reasons.
	// Coalescing would make unticking impossible — an empty set would read as
	// "not supplied" and leave the old one in place.
	const q = `
		UPDATE plg_playbook_run_task
		SET    value_bool    = CASE WHEN $2 THEN NULL ELSE COALESCE($3, value_bool)   END,
		       value_number  = CASE WHEN $2 THEN NULL ELSE COALESCE($4, value_number) END,
		       value_text    = CASE WHEN $2 THEN NULL ELSE COALESCE($5, value_text)   END,
		       value_checked = CASE WHEN $2 THEN NULL
		                            WHEN $6::TEXT[] IS NOT NULL THEN NULLIF($6::TEXT[], '{}')
		                            ELSE value_checked END
		WHERE  id::TEXT = $1
		RETURNING is_completed`

	// Both writes go in one transaction. is_completed is generated from the
	// value, so the first statement decides it and the second has to agree; if
	// the second failed on its own the task would read completed with no
	// completed_on — exactly the split state the comment below rules out.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", "", fmt.Errorf("begin patch run task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var completed bool
	if err := tx.QueryRow(ctx, q, req.ID, req.ClearValue,
		req.BoolValue, req.NumberValue, req.TextValue, req.CheckedCodes).Scan(&completed); err != nil {
		if isConstraintViolation(err) {
			return "", "", &apierror.ValidationError{Msg: "that value does not match the task's type"}
		}
		return "", "", fmt.Errorf("patch run task: %w", err)
	}

	// Stamp or clear the completion trail to match what the generated column now
	// says, so the two can never describe different states.
	if _, err := tx.Exec(ctx, `
		UPDATE plg_playbook_run_task
		SET    completed_on = CASE WHEN is_completed THEN COALESCE(completed_on, NOW()) ELSE NULL END,
		       completed_by = CASE WHEN is_completed THEN COALESCE(completed_by, $2::UUID) ELSE NULL END
		WHERE  id::TEXT = $1`, req.ID, uuidArg(actor)); err != nil {
		return "", "", actorWrite(err, actor, "stamp completion")
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("commit patch run task: %w", err)
	}

	return r.LocatePairing(ctx, pairingID)
}

func (r *orgPlatformRepository) CreateNote(ctx context.Context, req domain.CreateNoteRequest, actor string) error {
	pairingID, err := r.resolve(ctx, req.OrganizationID, req.ProductCode)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO plg_note (org_platform_id, body, author)
		VALUES ($1::UUID, $2, $3::UUID)`, pairingID, req.Body, uuidArg(actor)); err != nil {
		return actorWrite(err, actor, "create note")
	}
	return nil
}

// UpdateNote corrects a note's wording, keeping the superseded body.
//
// GUARDED WRITE (contract W6). Only the author may edit, and authorship is a
// WHERE clause rather than a read before the write — so the check and the write
// cannot be separated by anything.
//
// Order matters: the revision holds the body as it was BEFORE this edit, so
// reading revisions in edited_on order replays the note's history.
func (r *orgPlatformRepository) UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actor string) (domain.WriteResult, string, string, error) {
	var res domain.WriteResult

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return res, "", "", fmt.Errorf("begin update note: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pairingID, previous string
	err = tx.QueryRow(ctx, `
		SELECT org_platform_id::TEXT, body FROM plg_note WHERE id::TEXT = $1`,
		req.ID).Scan(&pairingID, &previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, "", "", &apierror.NotFoundError{Msg: "note not found"}
	}
	if err != nil {
		return res, "", "", fmt.Errorf("locate note: %w", err)
	}

	orgID, code, err := r.LocatePairing(ctx, pairingID)
	if err != nil {
		return res, "", "", err
	}

	// Nothing to record when the wording has not moved. Returning early keeps
	// plg_note_revision free of rows that say "this changed to what it already
	// was", and leaves updated_on reflecting the last real edit. Reported as
	// applied: the note says what the caller asked it to say.
	if previous == req.Body {
		res.RowsAffected = 1
		return res, orgID, code, nil
	}

	// The guard and the write in one statement: authorship is a WHERE clause.
	tag, err := tx.Exec(ctx, `
		UPDATE plg_note
		SET    body = $2, updated_on = NOW(), updated_by = $3::UUID
		WHERE  id::TEXT = $1 AND author = $3::UUID`,
		req.ID, req.Body, uuidArg(actor))
	if err != nil {
		return res, "", "", actorWrite(err, actor, "update note")
	}
	if tag.RowsAffected() == 0 {
		// Not the author. RowsAffected stays 0 and the BFF answers 403.
		return res, orgID, code, nil
	}

	// The body as it was, before the update above overwrote it.
	if _, err := tx.Exec(ctx, `
		INSERT INTO plg_note_revision (note_id, body, edited_by)
		VALUES ($1::UUID, $2, $3::UUID)`, req.ID, previous, uuidArg(actor)); err != nil {
		return res, "", "", actorWrite(err, actor, "record note revision")
	}

	if err := tx.Commit(ctx); err != nil {
		return res, "", "", fmt.Errorf("commit update note: %w", err)
	}
	res.RowsAffected = 1
	return res, orgID, code, nil
}

// SearchRegistrations backs the New Registrations panel. "New" is the absence of
// acknowledged_on — nothing stores a flag.
// buildRegistrationConditions filters the registrations list.
//
// It cannot share buildConditions with the organisation list, for two reasons
// that both bite:
//
//   - This list is of *pairings*, so "platform = IAM" must match the row's own
//     platform. The organisation list matches when any pairing matches, which is
//     right there and wrong here — it would show a customer's API Platform
//     registration when you filtered for IAM.
//   - The two queries read different views. plg_org_platform_v has no
//     created_on; the registration date it exposes is registered_on.
func buildRegistrationConditions(f *domain.OrganizationSearchFilters, b *argBuilder) []string {
	var conds []string

	if q := strings.TrimSpace(f.Query); q != "" {
		p := b.add("%" + strings.ToLower(q) + "%")
		conds = append(conds, `(LOWER(v.organization_name) LIKE `+p+
			` OR LOWER(v.registered_email) LIKE `+p+`)`)
	}
	if v := strings.TrimSpace(f.OrganizationName); v != "" {
		conds = append(conds, `LOWER(v.organization_name) LIKE `+b.add("%"+strings.ToLower(v)+"%"))
	}
	if v := strings.TrimSpace(f.RegisteredEmail); v != "" {
		conds = append(conds, `LOWER(v.registered_email) LIKE `+b.add("%"+strings.ToLower(v)+"%"))
	}
	if len(f.ProductCodes) > 0 {
		conds = append(conds, `v.product_code = ANY(`+b.add(f.ProductCodes)+`::TEXT[])`)
	}
	if len(f.LifecycleStages) > 0 {
		conds = append(conds, `v.lifecycle_stage::TEXT = ANY(`+
			b.add(stringsOf(f.LifecycleStages))+`::TEXT[])`)
	}
	if len(f.SubscriptionTiers) > 0 {
		conds = append(conds, `v.subscription_tier::TEXT = ANY(`+
			b.add(stringsOf(f.SubscriptionTiers))+`::TEXT[])`)
	}
	if len(f.OwnerIDs) > 0 {
		conds = append(conds, `v.plg_cs_owner::TEXT = ANY(`+b.add(f.OwnerIDs)+`::TEXT[])`)
	}
	if f.Unowned != nil && *f.Unowned {
		conds = append(conds, `v.plg_cs_owner IS NULL`)
	}
	if f.RegisteredFrom != nil {
		conds = append(conds, `v.registered_on >= `+b.add(*f.RegisteredFrom))
	}
	if f.RegisteredTo != nil {
		conds = append(conds, `v.registered_on <= `+b.add(*f.RegisteredTo))
	}
	return conds
}

func (r *orgPlatformRepository) SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) ([]domain.RegistrationItem, int, error) {
	b := &argBuilder{}
	conds := buildRegistrationConditions(&req.Filters, b)
	if req.UnacknowledgedOnly {
		conds = append(conds, `v.is_new`)
	}

	limit := b.add(req.Pagination.Limit)
	offset := b.add(req.Pagination.Offset)

	q := `
		SELECT v.org_platform_id, v.organization_id, v.organization_name,
		       v.product_id, v.product_code, v.product_name,
		       v.registered_email, v.registered_name, v.registered_on,
		       v.lifecycle_stage::TEXT, v.lifecycle_stage_name,
		       v.plg_cs_owner::TEXT, v.plg_cs_owner_email, v.plg_cs_owner_name,
		       COUNT(*) OVER() AS total
		FROM   plg_org_platform_v v` +
		whereClause(conds) + `
		ORDER  BY v.registered_on DESC
		LIMIT  ` + limit + ` OFFSET ` + offset

	rows, err := r.db.Query(ctx, q, b.list()...)
	if err != nil {
		return nil, 0, fmt.Errorf("search registrations: %w", err)
	}
	defer rows.Close()

	items := []domain.RegistrationItem{}
	total := 0
	for rows.Next() {
		var (
			it                             domain.RegistrationItem
			prodID, prodCode, prodName     string
			ownerID, ownerEmail, ownerName *string
		)
		if err := rows.Scan(&it.OrgPlatformID, &it.OrganizationID, &it.OrganizationName,
			&prodID, &prodCode, &prodName,
			&it.RegisteredEmail, &it.RegisteredName, &it.RegisteredOn,
			&it.LifecycleStage, &it.StageName, &ownerID, &ownerEmail, &ownerName, &total); err != nil {
			return nil, 0, fmt.Errorf("scan registration: %w", err)
		}
		it.Product = productRef(prodID, prodCode, prodName)
		it.Owner = userRef(ownerID, ownerEmail, ownerName)
		items = append(items, it)
	}
	return items, total, rows.Err()
}
