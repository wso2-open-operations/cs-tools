// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// StandardChangeRepository stores standard change templates, their CAB-approved
// versions and the proposals that create, modify or retire them (migration 0252).
// Every table it touches is internal-only (row-level security, migration 100026),
// and its routes are internal callers only: no customer identity reaches it.
//
// The repository enforces what must hold under a row lock (who owns a draft, which
// state an action needs, that a modification still starts from the current
// version); the service validates content and shapes requests before calling it.
type StandardChangeRepository interface {
	// SearchTemplates lists templates in the given (upper-case) states, by name,
	// with their current version and usage; accountID ("" for any) narrows them
	// to one customer's.
	SearchTemplates(ctx context.Context, states []string, query, accountID string, limit, offset int) ([]domain.StandardChangeTemplate, int, error)
	// GetTemplate returns one template with its current version, usage and
	// version history; nil when there is none with that id.
	GetTemplate(ctx context.Context, id string) (*domain.StandardChangeTemplate, error)
	// GetVersion returns one version; nil when there is none with that id.
	GetVersion(ctx context.Context, id string) (*domain.StandardChangeTemplateVersion, error)
	// SetTemplateState moves a template from one of from to to (suspend /
	// resume); a ConflictError when it is in none of them. Moving to SUSPENDED
	// records reason (nil for none), when and by whom; any other move clears them.
	SetTemplateState(ctx context.Context, id, actor string, from []string, to string, reason *string) (*domain.StandardChangeTemplate, error)
	// ClearReviewRequired clears the "review required" flag.
	ClearReviewRequired(ctx context.Context, id, actor string) (*domain.StandardChangeTemplate, error)
	// FlagReviewRequired raises the "review required" flag of the template a
	// change request was raised from, if it was raised from one, recording the
	// change request and why. Safe to call on any change request.
	FlagReviewRequired(ctx context.Context, tx pgx.Tx, changeRequestID, reason, actor string) error
	// IsCABMember reports whether the email is an active internal member of the
	// "CAB Approval" group.
	IsCABMember(ctx context.Context, email string) (bool, error)
	// ProjectAccount returns the customer (account) of a project: nil when there
	// is no such project, an empty EntityRef when it has no account.
	ProjectAccount(ctx context.Context, projectID string) (*domain.EntityRef, error)

	// CreateProposal writes a new draft and returns its id.
	CreateProposal(ctx context.Context, actor string, in StandardChangeProposalCreate) (string, error)
	// GetProposal returns one proposal; nil when there is none with that id.
	GetProposal(ctx context.Context, id string) (*domain.StandardChangeProposal, error)
	// SearchProposals lists proposals, newest first.
	SearchProposals(ctx context.Context, f StandardChangeProposalFilter) ([]domain.StandardChangeProposalSummary, int, error)
	// UpdateProposal edits a draft; only its proposer may.
	UpdateProposal(ctx context.Context, id, actor string, req domain.UpdateStandardChangeProposalRequest) error
	// SubmitProposal sends a draft to the CAB: check runs on the proposal read
	// under the row lock, then a CAB Approval stage is provisioned.
	SubmitProposal(ctx context.Context, id, actor string, check func(domain.StandardChangeProposal) error) error
	// WithdrawProposal withdraws a draft or a proposal in CAB review.
	WithdrawProposal(ctx context.Context, id, actor string) error
	// DecideProposal records a CAB member's decision; an approval applies the
	// proposal (new template, new version, or retirement) in the same transaction.
	DecideProposal(ctx context.Context, id, actor string, approve bool, comment *string) error
}

// StandardChangeProposalCreate is what CreateProposal writes.
type StandardChangeProposalCreate struct {
	Kind                  domain.StandardChangeProposalKind
	Name                  string
	TemplateID            *string
	SourceChangeRequestID *string
	CopyFromProposalID    *string
	// AccountID is the customer the caller named (nil for none); see
	// domain.CreateStandardChangeProposalRequest.AccountID.
	AccountID *string
}

// StandardChangeProposalFilter narrows SearchProposals.
type StandardChangeProposalFilter struct {
	States          []string // upper case
	Kinds           []string // upper case
	TemplateID      *string
	AccountID       *string
	ProposedByEmail string // "" = anyone
	AwaitingEmail   string // "" = no filter: proposals in CAB review with a REQUESTED row for this email
	Query           string
	Limit, Offset   int
}

type standardChangeRepo struct {
	db *Scoped
}

// NewStandardChangeRepository constructs a Postgres-backed StandardChangeRepository.
func NewStandardChangeRepository(db *Scoped) StandardChangeRepository {
	return &standardChangeRepo{db: db}
}

// standardChangeStageLabel is the label of a proposal's one approval stage.
const standardChangeStageLabel = approvalStageLabelCAB

// standardChangeCABCheckpoint provisions a proposal's stage: the CAB group, the
// proposer listed cancelled (nobody approves their own proposal).
var standardChangeCABCheckpoint = changeRequestApprovalCheckpoint{Position: 0, Label: standardChangeStageLabel, Pool: poolNamedGroup, GroupName: domain.CABApprovalGroupName}

// contentColumns are the template content columns, shared by the proposal and the
// version tables, in the order scanContent reads them.
const contentColumns = `subject, description, justification, implementation_plan, risk_impact_analysis,
	backout_plan, test_plan, category, impact, risk, rollback_duration_text,
	is_planning_visible_to_customers, customer_approval_required, customer_review_required, parameters`

func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

// contentDest returns scan destinations for contentColumns and a finish func
// that decodes them into c.
func contentDest(c *domain.StandardChangeContent) ([]any, func() error) {
	var category, impact, risk *string
	// The booleans go through nullable locals: a template whose current version
	// is missing (LEFT JOIN, templateSelect) reads all of them as NULL.
	var planningVisible, approvalRequired, reviewRequired *bool
	var params []byte
	dest := []any{&c.Subject, &c.Description, &c.Justification, &c.ImplementationPlan, &c.RiskImpactAnalysis,
		&c.BackoutPlan, &c.TestPlan, &category, &impact, &risk, &c.RollbackDurationText,
		&planningVisible, &approvalRequired, &reviewRequired, &params}
	return dest, func() error {
		c.IsPlanningVisibleToCustomers = planningVisible != nil && *planningVisible
		c.CustomerApprovalRequired = approvalRequired != nil && *approvalRequired
		c.CustomerReviewRequired = reviewRequired != nil && *reviewRequired
		if category != nil {
			v := domain.ChangeRequestCategory(*category)
			c.Category = &v
		}
		if impact != nil {
			v := domain.ChangeRequestImpact(*impact)
			c.Impact = &v
		}
		if risk != nil {
			v := domain.ChangeRequestRisk(*risk)
			c.Risk = &v
		}
		c.Parameters = []domain.StandardChangeParameter{}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &c.Parameters); err != nil {
				return fmt.Errorf("decode standard change parameters: %w", err)
			}
		}
		return nil
	}
}

func contentArgs(c domain.StandardChangeContent) ([]any, error) {
	params := c.Parameters
	if params == nil {
		params = []domain.StandardChangeParameter{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode standard change parameters: %w", err)
	}
	var category, impact, risk *string
	if c.Category != nil {
		v := string(*c.Category)
		category = &v
	}
	if c.Impact != nil {
		v := string(*c.Impact)
		impact = &v
	}
	if c.Risk != nil {
		v := string(*c.Risk)
		risk = &v
	}
	return []any{c.Subject, c.Description, c.Justification, c.ImplementationPlan, c.RiskImpactAnalysis,
		c.BackoutPlan, c.TestPlan, category, impact, risk, c.RollbackDurationText,
		c.IsPlanningVisibleToCustomers, c.CustomerApprovalRequired, c.CustomerReviewRequired, string(raw)}, nil
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

// templateSelect reads a template, its current version (LEFT: a template always
// has one, but a hand-made row might not) and its usage over every version.
const templateSelect = `SELECT t.id::text, t.number, t.name, t.state, acc.id::text, acc.name,
	ow.id::text, ow.email, COALESCE(NULLIF(ow.name, ''), NULLIF(btrim(COALESCE(ow.first_name, '') || ' ' || COALESCE(ow.last_name, '')), ''), ow.email),
	t.review_required, t.review_required_reason, t.review_required_on,
	t.suspended_reason, t.suspended_on, t.suspended_by,
	rcr.id::text, rwi.number,
	op.id::text, op.number,
	COALESCE(u.total, 0), COALESCE(u.open, 0), COALESCE(u.closed, 0), COALESCE(u.rolled_back, 0), COALESCE(u.canceled, 0), u.last_used_on,
	t.created_on, t.created_by, t.updated_on, t.updated_by,
	v.id::text, v.version, v.proposal_id::text, pwi.number, v.approved_on, v.approved_by, ` + "%s" + `
	FROM standard_change_template t
	LEFT JOIN account acc ON acc.id = t.account_id
	LEFT JOIN "user" ow ON ow.id = t.owner_user_id
	LEFT JOIN change_request rcr ON rcr.id = t.review_required_change_request_id
	LEFT JOIN work_item rwi ON rwi.id = rcr.id
	LEFT JOIN standard_change_template_version v ON v.id = t.current_version_id
	LEFT JOIN work_item pwi ON pwi.id = v.proposal_id
	LEFT JOIN LATERAL (
		SELECT p.id, w.number FROM standard_change_proposal p JOIN work_item w ON w.id = p.id
		 WHERE p.template_id = t.id AND p.kind IN ('MODIFY', 'RETIRE') AND p.state IN ('DRAFT', 'CAB_REVIEW')
		 LIMIT 1) op ON TRUE
	LEFT JOIN LATERAL (
		SELECT COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE cr.state IS NULL OR cr.state::text NOT IN ('CLOSED', 'CANCELED', 'ROLLBACK')) AS open,
		       COUNT(*) FILTER (WHERE cr.state::text = 'CLOSED') AS closed,
		       COUNT(*) FILTER (WHERE cr.state::text = 'ROLLBACK') AS rolled_back,
		       COUNT(*) FILTER (WHERE cr.state::text = 'CANCELED') AS canceled,
		       MAX(cwi.created_on) AS last_used_on
		  FROM change_request cr
		  JOIN work_item cwi ON cwi.id = cr.id
		  JOIN standard_change_template_version uv ON uv.id = cr.standard_change_template_version_id
		 WHERE uv.template_id = t.id) u ON TRUE`

func templateSelectSQL() string {
	return fmt.Sprintf(templateSelect, prefixed("v", contentColumns))
}

func scanTemplate(row pgx.Row) (domain.StandardChangeTemplate, error) {
	var t domain.StandardChangeTemplate
	var state string
	var accountID, accountName *string
	var ownerID, ownerEmail, ownerName *string
	var rcrID, rcrNumber, opID, opNumber *string
	var verID, verProposalID, verProposalNumber, verApprovedBy *string
	var verNo *int
	var verApprovedOn *time.Time
	var v domain.StandardChangeTemplateVersion
	cdest, finish := contentDest(&v.StandardChangeContent)
	dest := []any{&t.ID, &t.Number, &t.Name, &state, &accountID, &accountName,
		&ownerID, &ownerEmail, &ownerName,
		&t.ReviewRequired, &t.ReviewRequiredReason, &t.ReviewRequiredOn,
		&t.SuspendedReason, &t.SuspendedOn, &t.SuspendedBy,
		&rcrID, &rcrNumber, &opID, &opNumber,
		&t.Usage.Total, &t.Usage.Open, &t.Usage.Closed, &t.Usage.RolledBack, &t.Usage.Canceled, &t.Usage.LastUsedOn,
		&t.CreatedOn, &t.CreatedBy, &t.UpdatedOn, &t.UpdatedBy,
		&verID, &verNo, &verProposalID, &verProposalNumber, &verApprovedOn, &verApprovedBy}
	if err := row.Scan(append(dest, cdest...)...); err != nil {
		return t, err
	}
	t.State = domain.StandardChangeTemplateState(strings.ToLower(state))
	if t.State != domain.StandardChangeTemplateStateSuspended {
		// Every write that leaves SUSPENDED clears them; this covers a row a
		// hand-made update left behind.
		t.SuspendedReason, t.SuspendedOn, t.SuspendedBy = nil, nil, nil
	}
	t.Customer = accountRef(accountID, accountName)
	if ownerID != nil {
		t.Owner = domain.NewUserReference(*ownerID, stringOrEmpty(ownerEmail), stringOrEmpty(ownerName))
	}
	if rcrID != nil {
		t.ReviewRequiredChangeRequest = &domain.EntityRef{ID: *rcrID, Name: stringOrEmpty(rcrNumber)}
	}
	if opID != nil {
		t.OpenProposal = &domain.EntityRef{ID: *opID, Name: stringOrEmpty(opNumber)}
	}
	if verID != nil {
		if err := finish(); err != nil {
			return t, err
		}
		v.ID, v.TemplateID = *verID, t.ID
		if verNo != nil {
			v.Version = *verNo
		}
		if verProposalID != nil {
			v.Proposal = &domain.EntityRef{ID: *verProposalID, Name: stringOrEmpty(verProposalNumber)}
		}
		if verApprovedOn != nil {
			v.ApprovedOn = *verApprovedOn
		}
		v.ApprovedBy = stringOrEmpty(verApprovedBy)
		t.CurrentVersion = &v
	}
	return t, nil
}

// crvis: internal callers only (internalOnly routes; the template tables are internal-only RLS); counts change requests, exposes none
func (r *standardChangeRepo) SearchTemplates(ctx context.Context, states []string, query, accountID string, limit, offset int) ([]domain.StandardChangeTemplate, int, error) {
	where := `WHERE t.state = ANY($1::text[]) AND ($2 = '' OR t.name ILIKE '%' || $2 || '%' ESCAPE '\' OR t.number ILIKE '%' || $2 || '%' ESCAPE '\')
	  AND ($3 = '' OR t.account_id = NULLIF($3, '')::uuid)`
	q := escapeLikeWildcards(strings.TrimSpace(query))
	acc := strings.ToLower(strings.TrimSpace(accountID))
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM standard_change_template t `+where, states, q, acc).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search standard change templates: count: %w", err)
	}
	rows, err := r.db.Query(ctx, templateSelectSQL()+" "+where+` ORDER BY lower(t.name), t.id LIMIT $4 OFFSET $5`, states, q, acc, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search standard change templates: %w", err)
	}
	defer rows.Close()
	out := []domain.StandardChangeTemplate{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("search standard change templates: scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("search standard change templates: %w", err)
	}
	return out, total, nil
}

// crvis: internal callers only (internalOnly routes; the template tables are internal-only RLS); counts change requests, exposes none
func (r *standardChangeRepo) GetTemplate(ctx context.Context, id string) (*domain.StandardChangeTemplate, error) {
	t, err := scanTemplate(r.db.QueryRow(ctx, templateSelectSQL()+` WHERE t.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get standard change template: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT v.id::text, v.version, v.proposal_id::text, w.number, v.approved_on, v.approved_by
		  FROM standard_change_template_version v
		  LEFT JOIN work_item w ON w.id = v.proposal_id
		 WHERE v.template_id = $1::uuid
		 ORDER BY v.version DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("get standard change template: versions: %w", err)
	}
	defer rows.Close()
	t.Versions = []domain.StandardChangeTemplateVersionSummary{}
	for rows.Next() {
		var s domain.StandardChangeTemplateVersionSummary
		var pid, pnum *string
		if err := rows.Scan(&s.ID, &s.Version, &pid, &pnum, &s.ApprovedOn, &s.ApprovedBy); err != nil {
			return nil, fmt.Errorf("get standard change template: versions: scan: %w", err)
		}
		if pid != nil {
			s.Proposal = &domain.EntityRef{ID: *pid, Name: stringOrEmpty(pnum)}
		}
		t.Versions = append(t.Versions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get standard change template: versions: %w", err)
	}
	return &t, nil
}

func (r *standardChangeRepo) GetVersion(ctx context.Context, id string) (*domain.StandardChangeTemplateVersion, error) {
	return getVersion(ctx, r.db, id)
}

func getVersion(ctx context.Context, q crQuerier, id string) (*domain.StandardChangeTemplateVersion, error) {
	var v domain.StandardChangeTemplateVersion
	var pid, pnum *string
	cdest, finish := contentDest(&v.StandardChangeContent)
	err := q.QueryRow(ctx, `SELECT v.id::text, v.template_id::text, v.version, v.proposal_id::text, w.number, v.approved_on, v.approved_by, `+
		prefixed("v", contentColumns)+`
		  FROM standard_change_template_version v LEFT JOIN work_item w ON w.id = v.proposal_id
		 WHERE v.id = $1::uuid`, id).
		Scan(append([]any{&v.ID, &v.TemplateID, &v.Version, &pid, &pnum, &v.ApprovedOn, &v.ApprovedBy}, cdest...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get standard change template version: %w", err)
	}
	if err := finish(); err != nil {
		return nil, err
	}
	if pid != nil {
		v.Proposal = &domain.EntityRef{ID: *pid, Name: stringOrEmpty(pnum)}
	}
	return &v, nil
}

func (r *standardChangeRepo) SetTemplateState(ctx context.Context, id, actor string, from []string, to string, reason *string) (*domain.StandardChangeTemplate, error) {
	var current *string
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT state FROM standard_change_template WHERE id = $1::uuid FOR UPDATE`, id).Scan(&current); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &apierror.NotFoundError{Msg: "standard change template not found"}
			}
			return fmt.Errorf("set standard change template state: %w", err)
		}
		ok := false
		for _, f := range from {
			if *current == f {
				ok = true
			}
		}
		if !ok {
			return &apierror.ConflictError{Msg: fmt.Sprintf("the template is %s, so it cannot be moved to %s", strings.ToLower(*current), strings.ToLower(to))}
		}
		suspended := to == "SUSPENDED"
		if !suspended {
			reason = nil
		}
		_, err := tx.Exec(ctx, `UPDATE standard_change_template
			SET state = $2, updated_on = NOW(), updated_by = $3,
			    suspended_reason = $4,
			    suspended_on = CASE WHEN $5::boolean THEN NOW() END,
			    suspended_by = CASE WHEN $5::boolean THEN $3 END
			WHERE id = $1::uuid`, id, to, actor, reason, suspended)
		if err != nil {
			return templateWriteErr("set standard change template state", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetTemplate(ctx, id)
}

func (r *standardChangeRepo) ClearReviewRequired(ctx context.Context, id, actor string) (*domain.StandardChangeTemplate, error) {
	tag, err := r.db.Exec(ctx, `UPDATE standard_change_template
		SET review_required = false, review_required_reason = NULL, review_required_on = NULL,
		    review_required_change_request_id = NULL, updated_on = NOW(), updated_by = $2
		WHERE id = $1::uuid`, id, actor)
	if err != nil {
		return nil, templateWriteErr("clear standard change template review", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, &apierror.NotFoundError{Msg: "standard change template not found"}
	}
	return r.GetTemplate(ctx, id)
}

// FlagReviewRequired runs inside the caller's transaction, which may belong to a
// customer (their review answer can roll a change back), so it raises its own
// identity for the internal-only template table: the caller has already proven
// their access to the change request by writing it in this transaction.
//
// crvis: reads one change request by id inside a transaction that has already written it
func (r *standardChangeRepo) FlagReviewRequired(ctx context.Context, tx pgx.Tx, changeRequestID, reason, actor string) error {
	return flagStandardTemplateReview(ctx, tx, changeRequestID, reason, actor)
}

func flagStandardTemplateReview(ctx context.Context, tx pgx.Tx, changeRequestID, reason, actor string) error {
	var versionID *string
	if err := tx.QueryRow(ctx, `SELECT standard_change_template_version_id::text FROM change_request WHERE id = $1::uuid`, changeRequestID).Scan(&versionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("flag standard change template: %w", err)
	}
	if versionID == nil {
		return nil
	}
	if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
		return fmt.Errorf("flag standard change template: escalate identity: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE standard_change_template t
		   SET review_required = true, review_required_reason = $2, review_required_on = NOW(),
		       review_required_change_request_id = $3::uuid, updated_on = NOW(), updated_by = $4
		  FROM standard_change_template_version v
		 WHERE v.id = $1::uuid AND t.id = v.template_id
		   -- A template written before templates belonged to a customer cannot be
		   -- updated (migration 0254's check) until it is given one; flagging it
		   -- must not fail the change request's own write.
		   AND t.account_id IS NOT NULL
		   AND NOT (t.review_required AND t.review_required_change_request_id = $3::uuid AND t.review_required_reason = $2)`,
		*versionID, reason, changeRequestID, actor); err != nil {
		return fmt.Errorf("flag standard change template: %w", err)
	}
	return nil
}

// accountRef is a customer read through a LEFT JOIN: nil when there is none.
func accountRef(id, name *string) *domain.EntityRef {
	if id == nil {
		return nil
	}
	return &domain.EntityRef{ID: *id, Name: stringOrEmpty(name)}
}

func (r *standardChangeRepo) ProjectAccount(ctx context.Context, projectID string) (*domain.EntityRef, error) {
	return projectAccount(ctx, r.db, projectID)
}

// projectAccount is the customer of a project: nil when there is no such project,
// an empty EntityRef when the project has no account.
func projectAccount(ctx context.Context, q crQuerier, projectID string) (*domain.EntityRef, error) {
	var id, name *string
	err := q.QueryRow(ctx, `SELECT a.id::text, a.name FROM project p LEFT JOIN account a ON a.id = p.account_id WHERE p.id = $1::uuid`,
		strings.TrimSpace(projectID)).Scan(&id, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project customer: %w", err)
	}
	if ref := accountRef(id, name); ref != nil {
		return ref, nil
	}
	return &domain.EntityRef{}, nil
}

// readAccount is an account by id: nil when there is none.
func readAccount(ctx context.Context, q crQuerier, accountID string) (*domain.EntityRef, error) {
	var ref domain.EntityRef
	err := q.QueryRow(ctx, `SELECT id::text, name FROM account WHERE id = $1::uuid`, strings.TrimSpace(accountID)).Scan(&ref.ID, &ref.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read customer: %w", err)
	}
	return &ref, nil
}

// changeRequestAccount is the customer of a change request (its project's
// account): found reports whether the change request exists; the ref is nil
// when it has no project or the project no account.
func changeRequestAccount(ctx context.Context, q crQuerier, changeRequestID string) (ref *domain.EntityRef, found bool, err error) {
	var id, name *string
	err = q.QueryRow(ctx, `SELECT a.id::text, a.name FROM change_request cr JOIN work_item wi ON wi.id = cr.id
		LEFT JOIN project p ON p.id = wi.project_id LEFT JOIN account a ON a.id = p.account_id
		WHERE cr.id = $1::uuid`, changeRequestID).Scan(&id, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read change request customer: %w", err)
	}
	return accountRef(id, name), true, nil
}

func sameAccount(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// customerName names a customer in a message.
func customerName(ref *domain.EntityRef) string {
	if ref == nil || ref.ID == "" {
		return "no customer"
	}
	if ref.Name != "" {
		return ref.Name
	}
	return ref.ID
}

func (r *standardChangeRepo) IsCABMember(ctx context.Context, email string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM "user" u
		   WHERE LOWER(u.email) = LOWER($1) AND COALESCE(u.is_active, true) AND u.user_type::text = 'INTERNAL'
		     AND (u.id IN (SELECT tm.user_id FROM team_member tm
		                    WHERE tm.group_id IN (SELECT id FROM "group" WHERE name = $2)
		                       OR tm.team_id IN (SELECT id FROM team WHERE name = $2))
		       OR u.id IN (SELECT gm.user_id FROM group_member gm
		                    WHERE gm.group_id IN (SELECT id FROM "group" WHERE name = $2))))`,
		strings.TrimSpace(email), domain.CABApprovalGroupName).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check CAB membership: %w", err)
	}
	return ok, nil
}

// ---------------------------------------------------------------------------
// Proposals
// ---------------------------------------------------------------------------

// untitledStandardChangeName names a new proposal started with no name and nothing
// to take one from.
const untitledStandardChangeName = "Untitled standard change"

// CreateProposal opens a draft, copying a closed change request's content when it is named.
// crvis: internal callers only (internalOnly routes); reads the source change request to copy its plan into a proposal
func (r *standardChangeRepo) CreateProposal(ctx context.Context, actor string, in StandardChangeProposalCreate) (string, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		kind := strings.ToUpper(string(in.Kind))
		var content domain.StandardChangeContent
		var rationale *string
		var templateID, baseVersionID, sourceCR, copiedFrom *string
		// account is the customer the proposal is for (nil: a new draft that has
		// not named one yet).
		var account *string
		name := strings.TrimSpace(in.Name)
		evidence := []string{}
		// fixedCustomer refuses a named customer other than the one the proposal
		// takes from where it starts.
		fixedCustomer := func(from string, ref *domain.EntityRef) error {
			if in.AccountID == nil || (ref != nil && sameAccount(*in.AccountID, ref.ID)) {
				return nil
			}
			return &apierror.ValidationError{Msg: fmt.Sprintf("accountId must be %s's customer (%s): a standard change belongs to one customer, and %s cannot be used for another",
				from, customerName(ref), from)}
		}

		switch {
		case in.CopyFromProposalID != nil:
			src, err := getProposal(ctx, tx, *in.CopyFromProposalID)
			if err != nil {
				return "", err
			}
			if src == nil {
				return "", &apierror.ValidationError{Msg: "copyFromProposalId does not refer to a proposal"}
			}
			if src.Kind != domain.StandardChangeProposalKindNew {
				return "", &apierror.ValidationError{Msg: "only a proposal for a new template can be copied"}
			}
			if src.Customer != nil {
				if err := fixedCustomer("the copied proposal "+src.Number, src.Customer); err != nil {
					return "", err
				}
				account = &src.Customer.ID
			}
			content, rationale, copiedFrom = src.StandardChangeContent, src.Rationale, &src.ID
			if src.SourceChangeRequest != nil {
				// Carried over, and checked below against the copy's customer like
				// the evidence: a copy cites only its own customer's change requests.
				sourceCR = &src.SourceChangeRequest.ID
			}
			for _, e := range src.Evidence {
				evidence = append(evidence, e.ID)
			}
			if name == "" {
				name = src.Name
			}
		case in.Kind == domain.StandardChangeProposalKindNew && in.SourceChangeRequestID != nil:
			var state *string
			var subject string
			dest := []any{&state, &subject}
			var c domain.StandardChangeContent
			var category, impact, risk *string
			err := tx.QueryRow(ctx, `
				SELECT cr.state::text, wi.subject, wi.subject, wi.description, cr.justification, cr.implementation_plan,
				       COALESCE(cr.impact_description, cr.risk_impact_analysis), COALESCE(cr.rollback_process, cr.backout_plan),
				       cr.test_plan, LOWER(cr.category::text), LOWER(cr.impact::text), LOWER(cr.risk::text), cr.rollback_duration,
				       COALESCE(cr.is_planning_visible_to_customers, false), cr.customer_approval_required, cr.customer_review_required
				  FROM change_request cr JOIN work_item wi ON wi.id = cr.id WHERE cr.id = $1::uuid`, *in.SourceChangeRequestID).
				Scan(append(dest, &c.Subject, &c.Description, &c.Justification, &c.ImplementationPlan, &c.RiskImpactAnalysis,
					&c.BackoutPlan, &c.TestPlan, &category, &impact, &risk, &c.RollbackDurationText,
					&c.IsPlanningVisibleToCustomers, &c.CustomerApprovalRequired, &c.CustomerReviewRequired)...)
			if errors.Is(err, pgx.ErrNoRows) {
				return "", &apierror.ValidationError{Msg: "sourceChangeRequestId does not refer to a change request"}
			}
			if err != nil {
				return "", fmt.Errorf("create standard change proposal: read source change request: %w", err)
			}
			if state == nil || *state != crStateClosed {
				return "", &apierror.ValidationError{Msg: "a standard change can only be proposed from a change request that was completed (closed); this one is " + strings.ToLower(stringOrEmpty(state))}
			}
			cust, _, err := changeRequestAccount(ctx, tx, *in.SourceChangeRequestID)
			if err != nil {
				return "", err
			}
			if cust == nil {
				return "", &apierror.ValidationError{Msg: "a standard change belongs to one customer, and this change request has none (it is not raised for a customer's project), so it cannot be the start of one"}
			}
			if err := fixedCustomer("the change request", cust); err != nil {
				return "", err
			}
			account = &cust.ID
			if category != nil {
				v := domain.ChangeRequestCategory(*category)
				c.Category = &v
			}
			if impact != nil {
				v := domain.ChangeRequestImpact(*impact)
				c.Impact = &v
			}
			if risk != nil {
				v := domain.ChangeRequestRisk(*risk)
				c.Risk = &v
			}
			c.Parameters = []domain.StandardChangeParameter{}
			content, sourceCR = c, in.SourceChangeRequestID
			evidence = append(evidence, *in.SourceChangeRequestID)
			if name == "" {
				name = subject
			}
		case in.Kind == domain.StandardChangeProposalKindModify || in.Kind == domain.StandardChangeProposalKindRetire:
			var state, tname string
			var current, tplAccount, tplAccountName *string
			err := tx.QueryRow(ctx, `SELECT t.state, t.name, t.current_version_id::text, a.id::text, a.name
				FROM standard_change_template t LEFT JOIN account a ON a.id = t.account_id
				WHERE t.id = $1::uuid FOR SHARE OF t`, *in.TemplateID).Scan(&state, &tname, &current, &tplAccount, &tplAccountName)
			if errors.Is(err, pgx.ErrNoRows) {
				return "", &apierror.ValidationError{Msg: "templateId does not refer to a standard change template"}
			}
			if err != nil {
				return "", fmt.Errorf("create standard change proposal: read template: %w", err)
			}
			if state == "RETIRED" {
				return "", &apierror.ConflictError{Msg: "the template is retired"}
			}
			// A modification or retirement is for the template's customer.
			if err := fixedCustomer("the template", accountRef(tplAccount, tplAccountName)); err != nil {
				return "", err
			}
			account = tplAccount
			if current != nil {
				v, err := getVersion(ctx, tx, *current)
				if err != nil {
					return "", err
				}
				if v != nil {
					content = v.StandardChangeContent
				}
			}
			templateID, baseVersionID = in.TemplateID, current
			if name == "" {
				name = tname
			}
		}
		if account == nil && in.AccountID != nil {
			// A new proposal started from nothing (or copied from one that had no
			// customer) is for the customer it names.
			ref, err := readAccount(ctx, tx, *in.AccountID)
			if err != nil {
				return "", err
			}
			if ref == nil {
				return "", &apierror.ValidationError{Msg: "accountId does not refer to a customer"}
			}
			account = &ref.ID
		}
		// The source change request is a closed change request of the proposal's
		// customer (from a change request that was read above; for a copy, the
		// copied proposal's may have changed hands since it was cited).
		if err := checkSourceChangeRequestTx(ctx, tx, account, sourceCR); err != nil {
			return "", err
		}
		if name == "" {
			// A proposal started from nothing (the catalogue's "Propose a standard
			// change") opens under a placeholder the proposer renames in the editor.
			name = untitledStandardChangeName
		}
		if len([]rune(name)) > 255 {
			return "", &apierror.ValidationError{Msg: "name must be at most 255 characters"}
		}

		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, next_series_number('SCP'), $2, 'STANDARD_CHANGE_PROPOSAL'::work_item_type_enum)
			RETURNING id::text`, actor, name).Scan(&id); err != nil {
			return "", fmt.Errorf("create standard change proposal: work item: %w", err)
		}
		args, err := contentArgs(content)
		if err != nil {
			return "", err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO standard_change_proposal (id, kind, state, template_id, base_version_id, source_change_request_id, rationale, account_id, `+contentColumns+`, copied_from_proposal_id, customer_locked)
			VALUES ($1::uuid, $2::text, 'DRAFT', $3::uuid, $4::uuid, $5::uuid, $6, $7::uuid, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22::jsonb, $23::uuid,
			        $2::text <> 'NEW' OR $5::uuid IS NOT NULL OR ($23::uuid IS NOT NULL AND $7::uuid IS NOT NULL))`,
			append(append([]any{id, kind, templateID, baseVersionID, sourceCR, rationale, account}, args...), copiedFrom)...)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return "", &apierror.ConflictError{Msg: "this template already has a modification or retirement in draft or CAB review: continue that one, or withdraw it first"}
			}
			return "", fmt.Errorf("create standard change proposal: %w", err)
		}
		if err := setEvidenceTx(ctx, tx, id, actor, account, evidence); err != nil {
			return "", err
		}
		return id, nil
	})
}

// setEvidenceTx replaces a proposal's evidence: every id must be a closed change
// request of the proposal's customer (account; a 400 names those that are not).
func setEvidenceTx(ctx context.Context, tx pgx.Tx, proposalID, actor string, account *string, ids []string) error {
	clean, err := checkEvidenceTx(ctx, tx, account, ids)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM standard_change_proposal_evidence WHERE proposal_id = $1::uuid`, proposalID); err != nil {
		return fmt.Errorf("standard change evidence: clear: %w", err)
	}
	if len(clean) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO standard_change_proposal_evidence (proposal_id, change_request_id, created_by)
			SELECT $1::uuid, x, $2 FROM unnest($3::uuid[]) AS x`, proposalID, actor, clean); err != nil {
			return fmt.Errorf("standard change evidence: insert: %w", err)
		}
	}
	return nil
}

// checkEvidenceTx de-duplicates (lower-cased) evidence ids and refuses (400) any
// that is not a closed change request of the customer account: what the CAB
// pre-authorises for one customer is judged on that customer's changes. With no
// customer (a new draft that has not named one) no evidence can be cited.
//
// crvis: internal callers only; reads change request states and customers by id to validate evidence
func checkEvidenceTx(ctx context.Context, tx pgx.Tx, account *string, ids []string) ([]string, error) {
	seen := map[string]bool{}
	clean := []string{}
	for _, id := range ids {
		k := strings.ToLower(strings.TrimSpace(id))
		if k != "" && !seen[k] {
			seen[k] = true
			clean = append(clean, k)
		}
	}
	if len(clean) == 0 {
		return clean, nil
	}
	if account == nil {
		return nil, &apierror.ValidationError{Msg: "name the proposal's customer (accountId) before citing evidence: evidence is that customer's completed change requests"}
	}
	bad, err := changeRequestsNotClosedOfCustomer(ctx, tx, *account, clean, "standard change evidence")
	if err != nil {
		return nil, err
	}
	if len(bad) > 0 {
		return nil, &apierror.ValidationError{Msg: "evidence must be change requests of the proposal's customer that were completed (closed): " + strings.Join(bad, ", ")}
	}
	return clean, nil
}

// checkSourceChangeRequestTx refuses (400) a proposal's source change request
// (nil: none) that is not a closed change request of the proposal's customer
// account, in the evidence refusal's words. Checked when a proposal is created
// (from a change request, or a copy that carries the copied proposal's) and
// again at submit: a change request's customer is its project's account, and
// the Salesforce project sync may move a project to another account meanwhile.
//
// crvis: internal callers only; reads a change request's state and customer by id
func checkSourceChangeRequestTx(ctx context.Context, tx pgx.Tx, account, sourceCR *string) error {
	if sourceCR == nil {
		return nil
	}
	if account == nil {
		return &apierror.ValidationError{Msg: "name the proposal's customer (accountId): the change request it starts from must be that customer's"}
	}
	bad, err := changeRequestsNotClosedOfCustomer(ctx, tx, *account, []string{strings.ToLower(strings.TrimSpace(*sourceCR))}, "standard change source change request")
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return &apierror.ValidationError{Msg: "the change request a proposal starts from must be a change request of the proposal's customer that was completed (closed): " + strings.Join(bad, ", ")}
	}
	return nil
}

// checkProposalCustomerAtDecision refuses (409) the CAB's approval of a proposal whose
// source change request or evidence is no longer a closed change request of the
// proposal's customer, naming them. Nothing is written by the refused approval.
//
// crvis: internal callers only; reads a proposal's source and evidence change requests and their customers
func checkProposalCustomerAtDecision(ctx context.Context, tx pgx.Tx, proposalID string, account *string) error {
	var sourceCR *string
	if err := tx.QueryRow(ctx, `SELECT source_change_request_id::text FROM standard_change_proposal WHERE id = $1::uuid`, proposalID).Scan(&sourceCR); err != nil {
		return fmt.Errorf("check standard change proposal customer: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT change_request_id::text FROM standard_change_proposal_evidence WHERE proposal_id = $1::uuid`, proposalID)
	if err != nil {
		return fmt.Errorf("check standard change proposal customer: evidence: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("check standard change proposal customer: evidence: %w", err)
	}
	if account == nil && (sourceCR != nil || len(ids) > 0) {
		return &apierror.ConflictError{Msg: "the CAB cannot approve this proposal now: it names no customer, so its source change request and evidence cannot be checked; reject it, or ask the proposer to withdraw it and propose again"}
	}
	refuse := func(err error) error {
		var ve *apierror.ValidationError
		if errors.As(err, &ve) {
			return &apierror.ConflictError{Msg: "the CAB cannot approve this proposal now: " + ve.Msg + " (a project moved to another customer since it was submitted); reject it, or ask the proposer to withdraw it and propose again"}
		}
		return err
	}
	if err := checkSourceChangeRequestTx(ctx, tx, account, sourceCR); err != nil {
		return refuse(err)
	}
	if _, err := checkEvidenceTx(ctx, tx, account, ids); err != nil {
		return refuse(err)
	}
	return nil
}

// changeRequestsNotClosedOfCustomer names (number and why) each of ids (lower-case
// uuids) that is not a closed change request of the account.
//
// crvis: internal callers only; reads change request states and customers by id
func changeRequestsNotClosedOfCustomer(ctx context.Context, tx pgx.Tx, account string, clean []string, what string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT x.id::text, wi.number, cr.state::text, a.id::text, a.name
		  FROM unnest($1::uuid[]) AS x(id)
		  LEFT JOIN change_request cr ON cr.id = x.id
		  LEFT JOIN work_item wi ON wi.id = cr.id
		  LEFT JOIN project p ON p.id = wi.project_id
		  LEFT JOIN account a ON a.id = p.account_id`, clean)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	var bad []string
	for rows.Next() {
		var id string
		var number, state, accID, accName *string
		if err := rows.Scan(&id, &number, &state, &accID, &accName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		switch {
		case number == nil:
			bad = append(bad, id+" (not a change request)")
		case accID == nil:
			bad = append(bad, *number+" (no customer)")
		case !sameAccount(*accID, account):
			bad = append(bad, *number+" (another customer: "+customerName(accountRef(accID, accName))+")")
		case state == nil || *state != crStateClosed:
			bad = append(bad, *number+" ("+strings.ToLower(stringOrEmpty(state))+")")
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return bad, nil
}

// crvis: internal callers only (internalOnly routes); names evidence change requests by number
func (r *standardChangeRepo) GetProposal(ctx context.Context, id string) (*domain.StandardChangeProposal, error) {
	return getProposal(ctx, r.db, id)
}

// crvis: internal callers only; reads a proposal's own approval rows and evidence change requests
func getProposal(ctx context.Context, q crQuerier, id string) (*domain.StandardChangeProposal, error) {
	var p domain.StandardChangeProposal
	var kind, state string
	var tplID, tplName, srcID, srcNumber, decidedByID, decidedByEmail, decidedByName *string
	var accID, accName, copiedID, copiedNumber *string
	var proposerID, proposerName *string
	var proposerEmail string
	cdest, finish := contentDest(&p.StandardChangeContent)
	err := q.QueryRow(ctx, `
		SELECT wi.id::text, wi.number, wi.subject, p.kind, p.state, p.rationale,
		       t.id::text, COALESCE(t.number || ' ' || t.name, t.name), bv.version, scr.id::text, swi.number,
		       wi.created_by, pu.id::text, COALESCE(NULLIF(pu.name, ''), NULLIF(btrim(COALESCE(pu.first_name, '') || ' ' || COALESCE(pu.last_name, '')), '')),
		       wi.created_on, wi.updated_on, p.submitted_on, p.decided_on,
		       du.id::text, du.email, COALESCE(NULLIF(du.name, ''), du.email), p.decision_comment, rv.version,
		       acc.id::text, acc.name, cwi.id::text, cwi.number, `+prefixed("p", contentColumns)+`
		  FROM standard_change_proposal p
		  JOIN work_item wi ON wi.id = p.id
		  LEFT JOIN account acc ON acc.id = p.account_id
		  LEFT JOIN work_item cwi ON cwi.id = p.copied_from_proposal_id
		  LEFT JOIN standard_change_template t ON t.id = p.template_id
		  LEFT JOIN standard_change_template_version bv ON bv.id = p.base_version_id
		  LEFT JOIN standard_change_template_version rv ON rv.id = p.resulting_version_id
		  LEFT JOIN work_item swi ON swi.id = p.source_change_request_id
		  LEFT JOIN change_request scr ON scr.id = p.source_change_request_id
		  LEFT JOIN "user" du ON du.id = p.decided_by_user_id
		  LEFT JOIN LATERAL (SELECT u.id, u.name, u.first_name, u.last_name FROM "user" u
		                      WHERE LOWER(u.email) = LOWER(wi.created_by) ORDER BY u.id LIMIT 1) pu ON TRUE
		 WHERE p.id = $1::uuid`, id).
		Scan(append([]any{&p.ID, &p.Number, &p.Name, &kind, &state, &p.Rationale,
			&tplID, &tplName, &p.BaseVersion, &srcID, &srcNumber,
			&proposerEmail, &proposerID, &proposerName,
			&p.CreatedOn, &p.UpdatedOn, &p.SubmittedOn, &p.DecidedOn,
			&decidedByID, &decidedByEmail, &decidedByName, &p.DecisionNote, &p.ResultingVersion,
			&accID, &accName, &copiedID, &copiedNumber}, cdest...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get standard change proposal: %w", err)
	}
	if err := finish(); err != nil {
		return nil, err
	}
	p.Kind = domain.StandardChangeProposalKind(strings.ToLower(kind))
	p.State = domain.StandardChangeProposalState(strings.ToLower(state))
	p.Customer = accountRef(accID, accName)
	if tplID != nil {
		p.Template = &domain.EntityRef{ID: *tplID, Name: stringOrEmpty(tplName)}
	}
	if srcID != nil {
		p.SourceChangeRequest = &domain.EntityRef{ID: *srcID, Name: stringOrEmpty(srcNumber)}
	}
	if copiedID != nil {
		p.CopiedFrom = &domain.EntityRef{ID: *copiedID, Name: stringOrEmpty(copiedNumber)}
	}
	p.ProposedBy = domain.NewUserReference(stringOrEmpty(proposerID), proposerEmail, stringOrEmpty(proposerName))
	if decidedByID != nil {
		p.DecidedBy = domain.NewUserReference(*decidedByID, stringOrEmpty(decidedByEmail), stringOrEmpty(decidedByName))
	}

	p.Evidence = []domain.StandardChangeEvidence{}
	rows, err := q.Query(ctx, `
		SELECT cr.id::text, wi.number, wi.subject, LOWER(cr.state::text), cr.closed_on, ea.id::text, ea.name
		  FROM standard_change_proposal_evidence e
		  JOIN change_request cr ON cr.id = e.change_request_id
		  JOIN work_item wi ON wi.id = cr.id
		  LEFT JOIN project ep ON ep.id = wi.project_id
		  LEFT JOIN account ea ON ea.id = ep.account_id
		 WHERE e.proposal_id = $1::uuid
		 ORDER BY cr.closed_on DESC NULLS LAST, wi.number`, id)
	if err != nil {
		return nil, fmt.Errorf("get standard change proposal: evidence: %w", err)
	}
	for rows.Next() {
		var e domain.StandardChangeEvidence
		var eaID, eaName *string
		if err := rows.Scan(&e.ID, &e.Number, &e.Subject, &e.State, &e.ClosedOn, &eaID, &eaName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("get standard change proposal: evidence: scan: %w", err)
		}
		e.Customer = accountRef(eaID, eaName)
		p.Evidence = append(p.Evidence, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get standard change proposal: evidence: %w", err)
	}

	p.Approvers = []domain.StandardChangeProposalApprover{}
	rows, err = q.Query(ctx, `
		SELECT a.id::text, u.id::text, COALESCE(u.email, ''), COALESCE(NULLIF(u.name, ''), u.email, ''), a.state, a.comments, a.updated_on
		  FROM approval_stage_approver a
		  JOIN approval_stage s ON s.id = a.stage_id
		  LEFT JOIN "user" u ON u.id = a.approver_user_id
		 WHERE a.work_item_id = $1::uuid
		 ORDER BY s.created_on DESC, a.state = 'REQUESTED' DESC, lower(COALESCE(u.name, u.email, ''))`, id)
	if err != nil {
		return nil, fmt.Errorf("get standard change proposal: approvers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a domain.StandardChangeProposalApprover
		var uid *string
		var email, name string
		var st *string
		if err := rows.Scan(&a.ID, &uid, &email, &name, &st, &a.Comments, &a.UpdatedOn); err != nil {
			return nil, fmt.Errorf("get standard change proposal: approvers: scan: %w", err)
		}
		a.Approver = domain.NewUserReference(stringOrEmpty(uid), email, name)
		a.State = strings.ToUpper(stringOrEmpty(st))
		p.Approvers = append(p.Approvers, a)
	}
	return &p, rows.Err()
}

// SearchProposals lists proposals, newest first.
// crvis: internal callers only (internalOnly routes); reads the CAB rows of standard change proposals, never a change request's
func (r *standardChangeRepo) SearchProposals(ctx context.Context, f StandardChangeProposalFilter) ([]domain.StandardChangeProposalSummary, int, error) {
	where := `WHERE (cardinality($1::text[]) = 0 OR p.state = ANY($1::text[]))
	   AND (cardinality($2::text[]) = 0 OR p.kind = ANY($2::text[]))
	   AND ($3::uuid IS NULL OR p.template_id = $3::uuid)
	   AND ($4 = '' OR LOWER(wi.created_by) = LOWER($4))
	   AND ($5 = '' OR (p.state = 'CAB_REVIEW' AND EXISTS (
	         SELECT 1 FROM approval_stage_approver a JOIN "user" u ON u.id = a.approver_user_id
	          WHERE a.work_item_id = p.id AND a.state = 'REQUESTED' AND LOWER(u.email) = LOWER($5))))
	   AND ($6 = '' OR wi.subject ILIKE '%' || $6 || '%' ESCAPE '\' OR wi.number ILIKE '%' || $6 || '%' ESCAPE '\')
	   AND ($7::uuid IS NULL OR p.account_id = $7::uuid)`
	args := []any{f.States, f.Kinds, f.TemplateID, strings.TrimSpace(f.ProposedByEmail), strings.TrimSpace(f.AwaitingEmail), escapeLikeWildcards(strings.TrimSpace(f.Query)), f.AccountID}
	if f.States == nil {
		args[0] = []string{}
	}
	if f.Kinds == nil {
		args[1] = []string{}
	}
	from := ` FROM standard_change_proposal p JOIN work_item wi ON wi.id = p.id `
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search standard change proposals: count: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT wi.id::text, wi.number, p.kind, p.state, wi.subject, t.id::text, COALESCE(t.number || ' ' || t.name, t.name),
		       wi.created_by, pu.id::text, COALESCE(NULLIF(pu.name, ''), wi.created_by),
		       wi.created_on, wi.updated_on, p.submitted_on, p.decided_on, acc.id::text, acc.name`+from+`
		  LEFT JOIN standard_change_template t ON t.id = p.template_id
		  LEFT JOIN account acc ON acc.id = p.account_id
		  LEFT JOIN LATERAL (SELECT u.id, u.name FROM "user" u WHERE LOWER(u.email) = LOWER(wi.created_by) ORDER BY u.id LIMIT 1) pu ON TRUE
		`+where+` ORDER BY wi.created_on DESC, wi.id LIMIT $8 OFFSET $9`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("search standard change proposals: %w", err)
	}
	defer rows.Close()
	out := []domain.StandardChangeProposalSummary{}
	for rows.Next() {
		var s domain.StandardChangeProposalSummary
		var kind, state, email string
		var tid, tname, uid, uname, accID, accName *string
		if err := rows.Scan(&s.ID, &s.Number, &kind, &state, &s.Name, &tid, &tname, &email, &uid, &uname,
			&s.CreatedOn, &s.UpdatedOn, &s.SubmittedOn, &s.DecidedOn, &accID, &accName); err != nil {
			return nil, 0, fmt.Errorf("search standard change proposals: scan: %w", err)
		}
		s.Kind = domain.StandardChangeProposalKind(strings.ToLower(kind))
		s.State = domain.StandardChangeProposalState(strings.ToLower(state))
		s.Customer = accountRef(accID, accName)
		if tid != nil {
			s.Template = &domain.EntityRef{ID: *tid, Name: stringOrEmpty(tname)}
		}
		s.ProposedBy = domain.NewUserReference(stringOrEmpty(uid), email, stringOrEmpty(uname))
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// lockProposal locks a proposal's work item, then the proposal row (the order
// every writer takes), and returns its kind, state and proposer.
func lockProposal(ctx context.Context, tx pgx.Tx, id string) (kind, state, proposer string, err error) {
	if err = tx.QueryRow(ctx, `SELECT created_by FROM work_item WHERE id = $1::uuid AND type = 'STANDARD_CHANGE_PROPOSAL'::work_item_type_enum FOR NO KEY UPDATE`, id).Scan(&proposer); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", &apierror.NotFoundError{Msg: "standard change proposal not found"}
		}
		return "", "", "", fmt.Errorf("lock standard change proposal: %w", err)
	}
	if err = tx.QueryRow(ctx, `SELECT kind, state FROM standard_change_proposal WHERE id = $1::uuid FOR UPDATE`, id).Scan(&kind, &state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", &apierror.NotFoundError{Msg: "standard change proposal not found"}
		}
		return "", "", "", fmt.Errorf("lock standard change proposal: %w", err)
	}
	return kind, state, proposer, nil
}

// standardTemplateNoCustomerMsg is the refusal for changing a template that has no
// customer: one written before templates belonged to one (migration 0254), whose
// customer could not be told.
const standardTemplateNoCustomerMsg = "this standard change template has no customer recorded (it was written before templates belonged to one customer), so it cannot be changed until its customer is set"

// templateWriteErr maps a write of standard_change_template refused by migration
// 0254 (its NOT NULL, or on a database that still has templates without a
// customer its NOT VALID check) to a 409; anything else is wrapped.
func templateWriteErr(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && ((pgErr.Code == "23514" && pgErr.ConstraintName == "standard_change_template_account_required") ||
		(pgErr.Code == "23502" && pgErr.TableName == "standard_change_template" && pgErr.ColumnName == "account_id")) {
		return &apierror.ConflictError{Msg: standardTemplateNoCustomerMsg}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func sameEmail(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (r *standardChangeRepo) UpdateProposal(ctx context.Context, id, actor string, req domain.UpdateStandardChangeProposalRequest) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		kind, state, proposer, err := lockProposal(ctx, tx, id)
		if err != nil {
			return err
		}
		if !sameEmail(proposer, actor) {
			return &apierror.ForbiddenError{Msg: "only the proposer can edit a standard change proposal"}
		}
		if state != "DRAFT" {
			return &apierror.ConflictError{Msg: "only a draft can be edited; this proposal is " + strings.ToLower(state)}
		}
		if kind == "RETIRE" && hasContentEdits(req) {
			return &apierror.ValidationError{Msg: "a retirement proposal carries only its name, rationale and evidence"}
		}
		account, accountChanged, err := updateProposalCustomer(ctx, tx, id, kind, req.AccountID)
		if err != nil {
			return err
		}
		sets := []string{}
		args := []any{id}
		add := func(col string, v any, cast string) {
			args = append(args, v)
			sets = append(sets, fmt.Sprintf("%s = $%d%s", col, len(args), cast))
		}
		str := func(col string, v *string) {
			if v != nil {
				add(col, *v, "")
			}
		}
		str("rationale", req.Rationale)
		str("subject", req.Subject)
		str("description", req.Description)
		str("justification", req.Justification)
		str("implementation_plan", req.ImplementationPlan)
		str("risk_impact_analysis", req.RiskImpactAnalysis)
		str("backout_plan", req.BackoutPlan)
		str("test_plan", req.TestPlan)
		str("rollback_duration_text", req.RollbackDurationText)
		// An empty category, impact or risk clears it (the editor's "-- Select --").
		enumOrClear := func(col, v string) {
			if v == "" {
				sets = append(sets, col+" = NULL")
				return
			}
			add(col, v, "")
		}
		if req.Category != nil {
			enumOrClear("category", string(*req.Category))
		}
		if req.Impact != nil {
			enumOrClear("impact", string(*req.Impact))
		}
		if req.Risk != nil {
			enumOrClear("risk", string(*req.Risk))
		}
		if req.IsPlanningVisibleToCustomers != nil {
			add("is_planning_visible_to_customers", *req.IsPlanningVisibleToCustomers, "")
		}
		if req.CustomerApprovalRequired != nil {
			add("customer_approval_required", *req.CustomerApprovalRequired, "")
		}
		if req.CustomerReviewRequired != nil {
			add("customer_review_required", *req.CustomerReviewRequired, "")
		}
		if req.Parameters != nil {
			raw, err := json.Marshal(*req.Parameters)
			if err != nil {
				return fmt.Errorf("encode standard change parameters: %w", err)
			}
			add("parameters", string(raw), "::jsonb")
		}
		if len(sets) > 0 {
			if _, err := tx.Exec(ctx, `UPDATE standard_change_proposal SET `+strings.Join(sets, ", ")+` WHERE id = $1::uuid`, args...); err != nil {
				return fmt.Errorf("update standard change proposal: %w", err)
			}
		}
		wiSets := "updated_on = NOW(), updated_by = $2"
		wiArgs := []any{id, actor}
		if req.Name != nil {
			wiSets += ", subject = $3"
			wiArgs = append(wiArgs, *req.Name)
		}
		if _, err := tx.Exec(ctx, `UPDATE work_item SET `+wiSets+` WHERE id = $1::uuid`, wiArgs...); err != nil {
			return fmt.Errorf("update standard change proposal: work item: %w", err)
		}
		if accountChanged {
			// A copy names its customer once; a draft started from nothing may change it until it is submitted.
			if _, err := tx.Exec(ctx, `UPDATE standard_change_proposal SET account_id = $2::uuid, customer_locked = (copied_from_proposal_id IS NOT NULL) WHERE id = $1::uuid`, id, account); err != nil {
				return fmt.Errorf("update standard change proposal: customer: %w", err)
			}
		}
		if req.EvidenceChangeRequestIDs != nil {
			if err := setEvidenceTx(ctx, tx, id, actor, account, *req.EvidenceChangeRequestIDs); err != nil {
				return err
			}
		} else if accountChanged {
			// The evidence already cited must be the new customer's too.
			if _, err := checkEvidenceTx(ctx, tx, account, storedEvidence(ctx, tx, id)); err != nil {
				return err
			}
		}
		return nil
	})
}

// storedEvidence is the ids of a proposal's evidence (nil on a read error, which
// the caller's next statement reports).
func storedEvidence(ctx context.Context, tx pgx.Tx, proposalID string) []string {
	var ids []string
	_ = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(change_request_id::text), '{}') FROM standard_change_proposal_evidence WHERE proposal_id = $1::uuid`, proposalID).Scan(&ids)
	return ids
}

// updateProposalCustomer applies a PATCH's accountId to a draft (read under its
// lock): it returns the customer in effect afterwards and whether it changed. Only
// a new draft started from nothing (no source change request, not a copy) may
// change its customer; a copy of a proposal that had none may name one, once. On
// any other a different accountId is a 400 and resending its own is accepted.
func updateProposalCustomer(ctx context.Context, tx pgx.Tx, id, kind string, requested *string) (*string, bool, error) {
	var stored, sourceCR, sourceNumber, copiedFrom, copiedNumber *string
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT p.account_id::text, p.source_change_request_id::text, sw.number,
		       p.copied_from_proposal_id::text, cw.number, p.customer_locked
		FROM standard_change_proposal p LEFT JOIN work_item sw ON sw.id = p.source_change_request_id
		LEFT JOIN work_item cw ON cw.id = p.copied_from_proposal_id
		WHERE p.id = $1::uuid`, id).Scan(&stored, &sourceCR, &sourceNumber, &copiedFrom, &copiedNumber, &locked); err != nil {
		return nil, false, fmt.Errorf("update standard change proposal: read customer: %w", err)
	}
	if requested == nil || (stored != nil && sameAccount(*stored, *requested)) {
		return stored, false, nil
	}
	switch {
	case kind != "NEW":
		return nil, false, &apierror.ValidationError{Msg: "a modification or retirement is for its template's customer: accountId cannot change it"}
	case sourceCR != nil:
		return nil, false, &apierror.ValidationError{Msg: fmt.Sprintf("this proposal started from change request %s, so it is for that change request's customer: accountId cannot change it", stringOrEmpty(sourceNumber))}
	case copiedFrom != nil && stored != nil:
		return nil, false, &apierror.ValidationError{Msg: fmt.Sprintf("this proposal is a copy of proposal %s, so it is for that proposal's customer: accountId cannot change it; to propose it for another customer, start a new proposal", stringOrEmpty(copiedNumber))}
	case locked:
		// What it started from is gone (deleted), but its customer stays fixed.
		return nil, false, &apierror.ValidationError{Msg: "this proposal's customer is fixed (it started from a change request or a copy of another proposal): accountId cannot change it; to propose it for another customer, start a new proposal"}
	}
	ref, err := readAccount(ctx, tx, *requested)
	if err != nil {
		return nil, false, err
	}
	if ref == nil {
		return nil, false, &apierror.ValidationError{Msg: "accountId does not refer to a customer"}
	}
	return &ref.ID, true, nil
}

func hasContentEdits(req domain.UpdateStandardChangeProposalRequest) bool {
	return req.Subject != nil || req.Description != nil || req.Justification != nil || req.ImplementationPlan != nil ||
		req.RiskImpactAnalysis != nil || req.BackoutPlan != nil || req.TestPlan != nil || req.Category != nil ||
		req.Impact != nil || req.Risk != nil || req.RollbackDurationText != nil || req.IsPlanningVisibleToCustomers != nil ||
		req.CustomerApprovalRequired != nil || req.CustomerReviewRequired != nil || req.Parameters != nil
}

// checkTemplateStillCurrent refuses a modification or retirement whose template
// was retired, or (a modification) moved on to a version the draft did not start
// from, since the draft was opened.
func checkTemplateStillCurrent(ctx context.Context, tx pgx.Tx, proposalID string) error {
	var kind string
	var templateID, baseVersionID *string
	if err := tx.QueryRow(ctx, `SELECT kind, template_id::text, base_version_id::text FROM standard_change_proposal WHERE id = $1::uuid`, proposalID).
		Scan(&kind, &templateID, &baseVersionID); err != nil {
		return fmt.Errorf("check standard change template: %w", err)
	}
	if kind == "NEW" {
		return nil
	}
	if templateID == nil {
		return &apierror.ConflictError{Msg: "the template this proposal acts on no longer exists"}
	}
	var state string
	var current *string
	if err := tx.QueryRow(ctx, `SELECT state, current_version_id::text FROM standard_change_template WHERE id = $1::uuid FOR UPDATE`, *templateID).Scan(&state, &current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.ConflictError{Msg: "the template this proposal acts on no longer exists"}
		}
		return fmt.Errorf("check standard change template: %w", err)
	}
	if state == "RETIRED" {
		return &apierror.ConflictError{Msg: "the template has been retired"}
	}
	if kind == "MODIFY" && stringOrEmpty(current) != stringOrEmpty(baseVersionID) {
		return &apierror.ConflictError{Msg: "the template has a newer version than the one this modification started from: withdraw it and propose a modification of the current version"}
	}
	return nil
}

// crvis: internal callers only; provisions a CAB stage on a standard change proposal, never a change request
func (r *standardChangeRepo) SubmitProposal(ctx context.Context, id, actor string, check func(domain.StandardChangeProposal) error) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		_, state, proposer, err := lockProposal(ctx, tx, id)
		if err != nil {
			return err
		}
		if !sameEmail(proposer, actor) {
			return &apierror.ForbiddenError{Msg: "only the proposer can submit a standard change proposal"}
		}
		if state != "DRAFT" {
			return &apierror.ConflictError{Msg: "only a draft can be submitted; this proposal is " + strings.ToLower(state)}
		}
		p, err := getProposal(ctx, tx, id)
		if err != nil {
			return err
		}
		if p == nil {
			return &apierror.NotFoundError{Msg: "standard change proposal not found"}
		}
		if err := check(*p); err != nil {
			return err
		}
		if p.Customer == nil {
			// A new proposal names its customer (the service's completeness check
			// says so); a modification or retirement has its template's, which a
			// template written before templates belonged to a customer may lack.
			if p.Kind == domain.StandardChangeProposalKindNew {
				return &apierror.ValidationError{Msg: "the proposal is not complete; it still needs: customer (accountId)"}
			}
			return &apierror.ConflictError{Msg: standardTemplateNoCustomerMsg}
		}
		// The evidence was checked when it was cited; checked again against the
		// customer the CAB is asked about.
		ids := make([]string, 0, len(p.Evidence))
		for _, e := range p.Evidence {
			ids = append(ids, e.ID)
		}
		if _, err := checkEvidenceTx(ctx, tx, &p.Customer.ID, ids); err != nil {
			return err
		}
		// ... and so is the change request it started from (or that a copy carried
		// over): its project may have moved to another account since.
		if p.SourceChangeRequest != nil {
			if err := checkSourceChangeRequestTx(ctx, tx, &p.Customer.ID, &p.SourceChangeRequest.ID); err != nil {
				return err
			}
		}
		if err := checkTemplateStillCurrent(ctx, tx, id); err != nil {
			return err
		}
		// The name is checked here too, while the proposer can still rename the
		// draft; approval checks it again (a template may take the name meanwhile).
		if p.Kind != domain.StandardChangeProposalKindRetire {
			var taken bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM standard_change_template t
				                WHERE lower(btrim(t.name)) = lower(btrim($1)) AND t.state <> 'RETIRED'
				                  AND t.account_id = $3::uuid
				                  AND t.id IS DISTINCT FROM (SELECT template_id FROM standard_change_proposal WHERE id = $2::uuid))`,
				p.Name, id, p.Customer.ID).Scan(&taken); err != nil {
				return fmt.Errorf("submit standard change proposal: check name: %w", err)
			}
			if taken {
				return &apierror.ConflictError{Msg: fmt.Sprintf("%s already has a standard change template named %q: rename the proposal and submit it again", customerName(p.Customer), p.Name)}
			}
		}
		if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
			return fmt.Errorf("submit standard change proposal: escalate identity: %w", err)
		}
		creators, err := changeRequestCreatorUserIDs(ctx, tx, id)
		if err != nil {
			return err
		}
		pool, err := resolveApprovalPool(ctx, tx, standardChangeCABCheckpoint, nil, creators)
		if err != nil {
			return err
		}
		if err := insertApprovalStage(ctx, tx, id, actor, standardChangeStageLabel, pool, creators); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE standard_change_proposal SET state = 'CAB_REVIEW', submitted_on = NOW(), customer_locked = true WHERE id = $1::uuid`, id)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1::uuid`, id, actor)
		}
		if err != nil {
			return fmt.Errorf("submit standard change proposal: %w", err)
		}
		return nil
	})
}

// crvis: internal callers only; cancels the CAB rows of a standard change proposal, never a change request's
func (r *standardChangeRepo) WithdrawProposal(ctx context.Context, id, actor string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		_, state, proposer, err := lockProposal(ctx, tx, id)
		if err != nil {
			return err
		}
		if !sameEmail(proposer, actor) {
			return &apierror.ForbiddenError{Msg: "only the proposer can withdraw a standard change proposal"}
		}
		if state != "DRAFT" && state != "CAB_REVIEW" {
			return &apierror.ConflictError{Msg: "this proposal is already " + strings.ToLower(state)}
		}
		if err := cancelPendingApprovers(ctx, tx, id, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE standard_change_proposal SET state = 'WITHDRAWN', decided_on = NOW() WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("withdraw standard change proposal: %w", err)
		}
		_, err = tx.Exec(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1::uuid`, id, actor)
		return err
	})
}

// crvis: internal callers only; decides the CAB rows of a standard change proposal, never a change request's
func (r *standardChangeRepo) DecideProposal(ctx context.Context, id, actor string, approve bool, comment *string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		kind, state, proposer, err := lockProposal(ctx, tx, id)
		if err != nil {
			return err
		}
		if state != "CAB_REVIEW" {
			return &apierror.ConflictError{Msg: "this proposal is not waiting for the CAB: it is " + strings.ToLower(state)}
		}
		if sameEmail(proposer, actor) {
			return &apierror.ForbiddenError{Msg: "the proposer of a standard change cannot decide it"}
		}
		if err := setCallerIdentity(ctx, tx, SearchScope{Unrestricted: true}); err != nil {
			return fmt.Errorf("decide standard change proposal: escalate identity: %w", err)
		}
		var rowID, stageID, userID string
		err = tx.QueryRow(ctx, `
			SELECT a.id::text, a.stage_id::text, a.approver_user_id::text
			  FROM approval_stage_approver a JOIN "user" u ON u.id = a.approver_user_id
			 WHERE a.work_item_id = $1::uuid AND a.state = 'REQUESTED' AND LOWER(u.email) = LOWER($2)
			 ORDER BY a.created_on LIMIT 1 FOR UPDATE OF a`, id, actor).Scan(&rowID, &stageID, &userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.ForbiddenError{Msg: "only the CAB members asked to decide this standard change proposal can approve or reject it"}
		}
		if err != nil {
			return fmt.Errorf("decide standard change proposal: %w", err)
		}
		internal, err := internalApproverIDs(ctx, tx, []string{userID})
		if err != nil {
			return err
		}
		if !internal[strings.ToLower(userID)] {
			return &apierror.ForbiddenError{Msg: "only active internal (WSO2) users can decide a standard change proposal"}
		}
		decision := "REJECTED"
		if approve {
			decision = "APPROVED"
		}
		if _, err := tx.Exec(ctx, `UPDATE approval_stage_approver SET state = $2, comments = $3, updated_on = NOW(), updated_by = $4 WHERE id = $1::uuid`,
			rowID, decision, comment, actor); err != nil {
			return fmt.Errorf("decide standard change proposal: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE approval_stage_approver SET state = 'CANCELLED', updated_on = NOW(), updated_by = $2 WHERE stage_id = $1::uuid AND state = 'REQUESTED'`,
			stageID, actor); err != nil {
			return fmt.Errorf("decide standard change proposal: cancel the others: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE approval_stage SET raw_status = $2, updated_on = NOW(), updated_by = $3 WHERE id = $1::uuid`, stageID, decision, actor); err != nil {
			return fmt.Errorf("decide standard change proposal: stage: %w", err)
		}

		newState := "REJECTED"
		var resultingVersion, templateID *string
		if approve {
			newState = "APPROVED"
			templateID, resultingVersion, err = applyApprovedProposal(ctx, tx, id, kind, actor)
			if err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE standard_change_proposal
			SET state = $2, decided_on = NOW(), decided_by_user_id = $3::uuid, decision_comment = $4,
			    template_id = COALESCE($5::uuid, template_id), resulting_version_id = $6::uuid
			WHERE id = $1::uuid`, id, newState, userID, comment, templateID, resultingVersion); err != nil {
			return fmt.Errorf("decide standard change proposal: %w", err)
		}
		_, err = tx.Exec(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1::uuid`, id, actor)
		return err
	})
}

// applyApprovedProposal does what an approved proposal asks: a new template with
// version 1, a new version of an existing template (which clears its review flag),
// or a retirement.
func applyApprovedProposal(ctx context.Context, tx pgx.Tx, id, kind, actor string) (templateID, versionID *string, err error) {
	if err := checkTemplateStillCurrent(ctx, tx, id); err != nil {
		return nil, nil, err
	}
	var name, proposer string
	var tplID, account *string
	var c domain.StandardChangeContent
	cdest, finish := contentDest(&c)
	if err := tx.QueryRow(ctx, `SELECT wi.subject, wi.created_by, p.template_id::text, p.account_id::text, `+prefixed("p", contentColumns)+`
		FROM standard_change_proposal p JOIN work_item wi ON wi.id = p.id WHERE p.id = $1::uuid`, id).
		Scan(append([]any{&name, &proposer, &tplID, &account}, cdest...)...); err != nil {
		return nil, nil, fmt.Errorf("apply standard change proposal: %w", err)
	}
	if err := finish(); err != nil {
		return nil, nil, err
	}
	// The CAB pre-authorises the template for the proposal's customer on that
	// customer's changes: the source change request and the evidence are checked
	// again now, since the Salesforce project sync may have moved one of their
	// projects to another account after submit. A refusal changes nothing.
	if kind != "RETIRE" {
		if err := checkProposalCustomerAtDecision(ctx, tx, id, account); err != nil {
			return nil, nil, err
		}
	}
	nameTaken := func(err error) error {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return &apierror.ConflictError{Msg: fmt.Sprintf("this customer already has a standard change template named %q (it took the name after this proposal was submitted): reject it, or ask the proposer to withdraw it and propose again under another name", name)}
		}
		return templateWriteErr("apply standard change proposal", err)
	}

	switch kind {
	case "RETIRE":
		if _, err := tx.Exec(ctx, `UPDATE standard_change_template SET state = 'RETIRED', updated_on = NOW(), updated_by = $2,
			suspended_reason = NULL, suspended_on = NULL, suspended_by = NULL WHERE id = $1::uuid`, *tplID, actor); err != nil {
			return nil, nil, templateWriteErr("apply standard change proposal: retire", err)
		}
		return tplID, nil, nil
	case "NEW":
		if account == nil {
			return nil, nil, &apierror.ConflictError{Msg: "this proposal names no customer, and a standard change template belongs to one: reject it, or ask the proposer to withdraw it and propose again for a customer"}
		}
		var newID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO standard_change_template (name, owner_user_id, created_by, updated_by, account_id)
			VALUES ($1, (SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER($2) ORDER BY u.id LIMIT 1), $3, $3, $4::uuid)
			RETURNING id::text`, name, proposer, actor, *account).Scan(&newID); err != nil {
			return nil, nil, nameTaken(err)
		}
		tplID = &newID
	case "MODIFY":
		if _, err := tx.Exec(ctx, `UPDATE standard_change_template SET name = $2 WHERE id = $1::uuid`, *tplID, name); err != nil {
			return nil, nil, nameTaken(err)
		}
	}
	args, err := contentArgs(c)
	if err != nil {
		return nil, nil, err
	}
	var vid string
	if err := tx.QueryRow(ctx, `
		INSERT INTO standard_change_template_version (template_id, version, proposal_id, approved_by, `+contentColumns+`)
		VALUES ($1::uuid, COALESCE((SELECT MAX(version) FROM standard_change_template_version WHERE template_id = $1::uuid), 0) + 1,
		        $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18::jsonb)
		RETURNING id::text`, append([]any{*tplID, id, actor}, args...)...).Scan(&vid); err != nil {
		return nil, nil, fmt.Errorf("apply standard change proposal: version: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE standard_change_template
		   SET current_version_id = $2::uuid, review_required = false, review_required_reason = NULL,
		       review_required_on = NULL, review_required_change_request_id = NULL, updated_on = NOW(), updated_by = $3
		 WHERE id = $1::uuid`, *tplID, vid, actor); err != nil {
		return nil, nil, templateWriteErr("apply standard change proposal: current version", err)
	}
	return tplID, &vid, nil
}
