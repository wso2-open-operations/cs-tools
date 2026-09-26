package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// PlaybookRepository owns playbook templates — the Playbook Manager's data.
type PlaybookRepository interface {
	ListAll(ctx context.Context) ([]domain.Playbook, error)
	ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error)
	// ListForStage returns the active playbooks a pairing at this stage and
	// health may run.
	//
	// The kind is not optional. A pairing is offered progressive playbooks OR
	// risk-intervention ones, never both — there is no useful sense in which a
	// customer needs progressing and rescuing in the same moment — so the caller
	// passes which, and the caller derives it from health.
	ListForStage(ctx context.Context, productID string, stage domain.LifecycleStage, kinds []domain.PlaybookType) ([]domain.Playbook, error)
	Get(ctx context.Context, id string) (*domain.Playbook, error)
	Create(ctx context.Context, req domain.CreatePlaybookRequest) (string, error)
	Patch(ctx context.Context, req domain.PatchPlaybookRequest) error
	ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) error
	Delete(ctx context.Context, id string) error
}

type playbookRepository struct{ db *pgxpool.Pool }

// NewPlaybookRepository builds a PlaybookRepository over the given pool.
func NewPlaybookRepository(db *pgxpool.Pool) PlaybookRepository { return &playbookRepository{db: db} }

const playbookSelect = `
	SELECT pb.id::TEXT, p.id::TEXT, p.code, p.name,
	       pb.name, pb.description,
	       pb.lifecycle_stage::TEXT, ls.name, pb.playbook_type::TEXT,
	       pb.display_order, pb.active,
	       (SELECT COUNT(*)::INT FROM plg_playbook_run r WHERE r.playbook_id = pb.id),
	       (SELECT COUNT(*)::INT FROM plg_playbook_run_v v
	         WHERE v.playbook_id = pb.id AND v.run_status = 'ACTIVE'),
	       pb.created_at, pb.updated_at
	FROM   plg_playbook pb
	JOIN   plg_product p            ON p.id = pb.product_id
	JOIN   plg_lifecycle_stage ls   ON ls.stage = pb.lifecycle_stage`

// stageOrder sorts playbooks by the lifecycle order of their source stage, so
// the manager's sections come out in lifecycle order without the client sorting.
const stageOrder = `
	ORDER BY p.display_order,
	         (SELECT display_order FROM plg_lifecycle_stage WHERE stage = pb.lifecycle_stage),
	         pb.display_order, pb.name`

func (r *playbookRepository) list(ctx context.Context, q string, args ...any) ([]domain.Playbook, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list playbooks: %w", err)
	}

	items := []domain.Playbook{}
	ids := []string{}
	pos := map[string]int{}
	for rows.Next() {
		var pb domain.Playbook
		if err := rows.Scan(&pb.ID, &pb.Product.ID, &pb.Product.Code, &pb.Product.Name,
			&pb.Name, &pb.Description,
			&pb.LifecycleStage, &pb.StageName, &pb.PlaybookType,
			&pb.DisplayOrder, &pb.Active, &pb.RunCount, &pb.ActiveRuns,
			&pb.CreatedOn, &pb.UpdatedOn); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan playbook: %w", err)
		}
		pb.Tasks = []domain.PlaybookTask{}
		items = append(items, pb)
		ids = append(ids, pb.ID)
		pos[pb.ID] = len(items) - 1
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playbooks: %w", err)
	}
	if len(items) == 0 {
		return items, nil
	}

	taskRows, err := r.db.Query(ctx, `
		SELECT playbook_id::TEXT, id::TEXT, code, name, description, sequence_no,
		       value_type::TEXT, options, active
		FROM   plg_playbook_task
		WHERE  playbook_id::TEXT = ANY($1::TEXT[])
		ORDER  BY sequence_no`, ids)
	if err != nil {
		return nil, fmt.Errorf("list playbook tasks: %w", err)
	}
	defer taskRows.Close()

	for taskRows.Next() {
		var (
			pbID    string
			t       domain.PlaybookTask
			rawOpts []byte
		)
		if err := taskRows.Scan(&pbID, &t.ID, &t.Code, &t.Name, &t.Description,
			&t.SequenceNo, &t.ValueType, &rawOpts, &t.Active); err != nil {
			return nil, fmt.Errorf("scan playbook task: %w", err)
		}
		opts, err := scanOptions(rawOpts)
		if err != nil {
			return nil, err
		}
		t.Options = opts
		t.IsBookend = domain.IsBookendTask(t.Code)
		if i, ok := pos[pbID]; ok {
			items[i].Tasks = append(items[i].Tasks, t)
		}
	}
	return items, taskRows.Err()
}

func (r *playbookRepository) ListAll(ctx context.Context) ([]domain.Playbook, error) {
	return r.list(ctx, playbookSelect+stageOrder)
}

func (r *playbookRepository) ListByProduct(ctx context.Context, productCode string) ([]domain.Playbook, error) {
	return r.list(ctx, playbookSelect+` WHERE p.code = $1 OR p.id::TEXT = $1`+stageOrder, productCode)
}

func (r *playbookRepository) ListForStage(ctx context.Context, productID string, stage domain.LifecycleStage, kinds []domain.PlaybookType) ([]domain.Playbook, error) {
	// A set rather than a single kind since SUSTAINING arrived: a healthy
	// pairing is offered both progressive and sustaining work, so the menu asks
	// for two kinds at once. ANY over an array keeps it one query and one round
	// trip whichever way health falls.
	return r.list(ctx,
		playbookSelect+` WHERE p.id::TEXT = $1 AND pb.lifecycle_stage = $2::plg_lifecycle_stage_enum
		                   AND pb.playbook_type::TEXT = ANY($3::TEXT[]) AND pb.active`+stageOrder,
		productID, string(stage), stringsOf(kinds))
}

func (r *playbookRepository) Get(ctx context.Context, id string) (*domain.Playbook, error) {
	items, err := r.list(ctx, playbookSelect+` WHERE pb.id::TEXT = $1`+stageOrder, id)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, &apierror.NotFoundError{Msg: "playbook not found"}
	}
	return &items[0], nil
}

// Create inserts a playbook and its task list.
//
// The composite FK on (lifecycle_stage, target_stage) does the validation: a
// playbook at a stage that carries none, or aiming somewhere the paths do not
// allow, is refused by the database. That surfaces as a 400 with the reason.
func (r *playbookRepository) Create(ctx context.Context, req domain.CreatePlaybookRequest) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin create playbook: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID string
	err = tx.QueryRow(ctx, `SELECT id::TEXT FROM plg_product WHERE code = $1 OR id::TEXT = $1`,
		req.ProductCode).Scan(&productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &apierror.NotFoundError{Msg: "product not found"}
	}
	if err != nil {
		return "", fmt.Errorf("resolve product: %w", err)
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO plg_playbook (product_id, name, description, lifecycle_stage, playbook_type, display_order)
		VALUES ($1::UUID, $2, $3, $4::plg_lifecycle_stage_enum, $5::plg_playbook_type_enum,
		        (SELECT COALESCE(MAX(display_order), 0) + 1 FROM plg_playbook WHERE product_id = $1::UUID))
		RETURNING id::TEXT`,
		productID, req.Name, req.Description,
		string(req.LifecycleStage), string(req.PlaybookType)).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", &apierror.ConflictError{Msg: "a playbook with that name already exists for this product"}
		}
		if isConstraintViolation(err) {
			// The only check constraint left on this table refuses ABANDONED,
			// where neither kind of playbook could ever run.
			return "", &apierror.ValidationError{Msg: fmt.Sprintf(
				"a playbook cannot sit at %s: nothing progresses out of it and a pairing there is gone",
				req.LifecycleStage)}
		}
		return "", fmt.Errorf("create playbook: %w", err)
	}

	if err := insertTasks(ctx, tx, id, req.Tasks); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit create playbook: %w", err)
	}
	return id, nil
}

func insertTasks(ctx context.Context, tx pgx.Tx, playbookID string, tasks []domain.PlaybookTaskInput) error {
	for i, t := range tasks {
		opts, err := optionsArg(t.Options)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO plg_playbook_task (playbook_id, code, name, description, sequence_no, value_type, options)
			VALUES ($1::UUID, $2, $3, $4, $5, $6::plg_task_value_type_enum, $7::JSONB)`,
			playbookID, t.Code, t.Name, t.Description, i+1, string(t.ValueType), opts); err != nil {
			if isUniqueViolation(err) {
				return &apierror.ValidationError{Msg: "duplicate task code: " + t.Code}
			}
			if isConstraintViolation(err) {
				return &apierror.ValidationError{Msg: t.Code + " is a structural task and is always a tick box"}
			}
			return fmt.Errorf("create playbook task %s: %w", t.Code, err)
		}
	}
	return nil
}

func (r *playbookRepository) Patch(ctx context.Context, req domain.PatchPlaybookRequest) error {
	const q = `
		UPDATE plg_playbook
		SET    name            = COALESCE($2, name),
		       description     = COALESCE($3, description),
		       lifecycle_stage = COALESCE($4::plg_lifecycle_stage_enum, lifecycle_stage),
		       playbook_type   = COALESCE($5::plg_playbook_type_enum, playbook_type),
		       active          = COALESCE($6, active)
		WHERE  id::TEXT = $1`

	tag, err := r.db.Exec(ctx, q, req.ID, req.Name, req.Description,
		enumArg(req.LifecycleStage), enumArg(req.PlaybookType), req.Active)
	if err != nil {
		if isUniqueViolation(err) {
			return &apierror.ConflictError{Msg: "another playbook on this product already uses that name"}
		}
		if isConstraintViolation(err) {
			return &apierror.ValidationError{Msg: "a playbook cannot sit at ABANDONED"}
		}
		return fmt.Errorf("patch playbook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "playbook not found"}
	}
	return nil
}

// ReplaceTasks swaps a playbook's task list wholesale.
//
// Runs already in flight are untouched: their task instances are copies, so a
// template edit reaches new runs only. That is the entire reconciliation policy —
// there is no prompt and no versioning, which means two pairings can be running
// visibly different versions of the same playbook.
//
// A task still referenced by a run cannot be deleted (the FK is ON DELETE
// RESTRICT), so it is deactivated instead. It stays out of new runs while the
// instances that point at it survive.
func (r *playbookRepository) ReplaceTasks(ctx context.Context, req domain.ReplacePlaybookTasksRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin replace tasks: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT TRUE FROM plg_playbook WHERE id::TEXT = $1 FOR UPDATE`,
		req.PlaybookID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "playbook not found"}
		}
		return fmt.Errorf("lock playbook: %w", err)
	}

	// Park every existing task out of the way first. sequence_no is UNIQUE per
	// playbook and DEFERRABLE, but codes are not — negative numbers keep the
	// upsert below free to reuse any position.
	if _, err := tx.Exec(ctx,
		`UPDATE plg_playbook_task SET sequence_no = -sequence_no, active = FALSE
		 WHERE playbook_id::TEXT = $1 AND sequence_no > 0`, req.PlaybookID); err != nil {
		return fmt.Errorf("park existing tasks: %w", err)
	}

	for i, t := range req.Tasks {
		opts, err := optionsArg(t.Options)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO plg_playbook_task (playbook_id, code, name, description, sequence_no, value_type, options, active)
			VALUES ($1::UUID, $2, $3, $4, $5, $6::plg_task_value_type_enum, $7::JSONB, TRUE)
			ON CONFLICT (playbook_id, code) DO UPDATE
			    SET name        = EXCLUDED.name,
			        description = EXCLUDED.description,
			        sequence_no = EXCLUDED.sequence_no,
			        value_type  = EXCLUDED.value_type,
			        options     = EXCLUDED.options,
			        active      = TRUE`,
			req.PlaybookID, t.Code, t.Name, t.Description, i+1, string(t.ValueType), opts); err != nil {
			if isConstraintViolation(err) {
				return &apierror.ValidationError{Msg: t.Code + " is a structural task and is always a tick box"}
			}
			return fmt.Errorf("upsert task %s: %w", t.Code, err)
		}
	}

	// Anything still parked was dropped from the list. Delete it if no run ever
	// used it; otherwise leave it deactivated so the instances keep their parent.
	if _, err := tx.Exec(ctx, `
		DELETE FROM plg_playbook_task t
		WHERE  t.playbook_id::TEXT = $1 AND t.sequence_no < 0
		  AND  NOT EXISTS (SELECT 1 FROM plg_playbook_run_task rt WHERE rt.playbook_task_id = t.id)`,
		req.PlaybookID); err != nil {
		return fmt.Errorf("delete removed tasks: %w", err)
	}

	return tx.Commit(ctx)
}

// Delete removes a playbook. Refused while any pairing still runs it — the FK is
// ON DELETE RESTRICT, and naming the count is more use than the raw error.
func (r *playbookRepository) Delete(ctx context.Context, id string) error {
	var inUse int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*)::INT FROM plg_playbook_run WHERE playbook_id::TEXT = $1`, id).Scan(&inUse); err != nil {
		return fmt.Errorf("count playbook usage: %w", err)
	}
	if inUse > 0 {
		return &apierror.ConflictError{Msg: fmt.Sprintf(
			"%d organisation+platform pairing(s) still run this playbook; remove it there or deactivate the playbook instead",
			inUse)}
	}

	tag, err := r.db.Exec(ctx, `DELETE FROM plg_playbook WHERE id::TEXT = $1`, id)
	if err != nil {
		return fmt.Errorf("delete playbook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "playbook not found"}
	}
	return nil
}
