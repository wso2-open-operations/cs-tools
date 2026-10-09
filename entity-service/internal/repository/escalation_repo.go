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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// EscalationRepository defines the persistence operations for case_escalation
// and case_escalation_notification_list (migration 0054).
//
// Both tables are project-membership-scoped by row-level security
// (migration 0141, updated by a later migration once CreateEscalation
// below turned out to be a genuine customer-facing write, not
// internal-only as first assumed), keyed on the caller identity Scoped
// forwards as session GUCs -- this repository does no project filtering of
// its own at all; a caller sees and writes exactly the rows Postgres
// decides to hand back.
type EscalationRepository interface {
	// SearchEscalations returns a filtered, sorted, paginated slice of
	// escalations together with the total count of matching rows before
	// pagination.
	SearchEscalations(ctx context.Context, caseIDs []string, currentLevels []int, sortField, sortOrder string, limit, offset int) ([]domain.Escalation, int, error)
	// CreateEscalation escalates or de-escalates caseID and recomputes its
	// notification-recipient list, all in one transaction.
	//
	// Level transition, as ServiceNow's EscalationUtils.createEscalation /
	// deescalateToEL0 (what the portal API's ESCALATE / DEESCALATE call):
	// ESCALATE sets current_level to the case's level + 1; on an EL5 case it
	// is a ConflictError (SN's 409 "Case already at maximum escalation
	// level"), and nothing is written. DEESCALATE always drops the case
	// straight to EL0, from whatever level it is at; on an EL0 case it is a
	// ConflictError (SN's 409 "Case already at EL0"). previous_level is
	// whatever the level was immediately before this call. "case".is_escalated
	// is TRUE for any resulting level >= 1, FALSE at EL0.
	//
	// Notification recipients: resolved cumulatively for EL1..N, where N is
	// the RESULTING level for ESCALATE and the level the case LEFT for
	// DEESCALATE (SN: "same list that was used when the case FIRST moved into
	// currentLevel") -- so de-escalating EL3 -> EL0 notifies the EL1..EL3
	// set, the people the escalation reached.
	// This ports ServiceNow's EscalationNotificationUtils.
	// resolveNotificationUsers (scoped app x_wso2_customer_0), step for step,
	// in its order, deduped by user id:
	//
	//   - EL1: the account's CRE team lead(s), then every
	//     notifyCfg.EL1AmericasTLEmails user (SN: "Append Americas TL users
	//     -- ALWAYS", no region gate), then account.account_manager_id (SN
	//     u_owner, Salesforce owner.email) and account.technical_owner_id
	//     (SN u_technical_owner). SN's team leads are the u_team_member_role
	//     rows on the account's u_integration_cs_team; here they are the
	//     team_member rows with role 'lead' on the team whose id is
	//     account.cre_team_id (team.id is the synced sys_user_group id, the
	//     same id cre_team_id holds; team_member.group_id is not reliably
	//     set, so the join is on team_id). An account whose CRE team has no
	//     team row, or no lead, contributes no team lead.
	//   - EL2: every notifyCfg.EL2AmericasTUEmails user, plus exactly one
	//     product contact picked from the case's product
	//     (deployed_product -> product): category SERVICE ->
	//     EL2ProductServiceEmail; SOFTWARE whose code is "wso2is" or whose
	//     name is "WSO2 Identity Server" -> EL2ProductIdentityServerEmail;
	//     any other product, AND a case with no product at all ->
	//     EL2ProductDefaultEmail (SN defaults rather than skipping).
	//   - EL3: every holder of the team position cre_head (team_member.role,
	//     the CRE head the incident call ladder also pages), plus
	//     account.customer_success_manager_id.
	//   - EL4: every holder of the app role case_escalation_el4 (SN: the CCO
	//     and CRO).
	//   - EL5: every holder of the app role case_escalation_el5 (SN: the CEO).
	//
	// EL3-EL5 are data, not configuration: who they reach changes by setting
	// a team position or granting a role (migration 0209 seeds the two
	// roles), never by redeploying. SN read them from its
	// x_wso2_customer_0.escalation.* properties. The EL1/EL2 notifyCfg emails
	// are still those properties, copied as they are; each resolves to a
	// "user" by email (or user_name, which is where SN looks). An unset slot,
	// an address with no user, or a position/role nobody holds contributes
	// nobody and is never an error -- SN logs and carries on the same way.
	//
	// Authorization ("only someone on the case's current notified-users list
	// may de-escalate") is deliberately NOT enforced here -- same reasoning
	// as CaseRepository.AcknowledgeCase's own doc comment: no Postgres-side
	// permission model exists yet, so this data source only requires a
	// known authenticated caller, same as every other Postgres case
	// mutation. A known, documented gap, not a silent omission.
	//
	// Returns a NotFoundError if caseID doesn't reference an existing case
	// (a work_item with no "case" extension row -- e.g. an engagement --
	// counts as not found here, since current_escalation_level/is_escalated
	// only ever live on "case").
	CreateEscalation(ctx context.Context, caseID string, action domain.EscalationAction, reason *string, actorEmail string) (domain.CreatedEscalation, error)
	// CaseTeamLeads returns the leads of caseID's account's CRE (ABT) team:
	// team_member rows with role 'lead' on the team whose id is
	// account.cre_team_id, oldest membership first -- the same people EL1
	// notifies as team leads. Empty, not an error, when the case has no
	// account, the account no CRE team, or the team no lead.
	CaseTeamLeads(ctx context.Context, caseID string) ([]domain.EscalationNotifiedUser, error)
}

// EscalationNotificationConfig holds the fixed EL1/EL2 recipients
// CreateEscalation layers on top of the per-case ones -- see that method's
// own doc comment for the full EL1..EL5 rule (EL3-EL5 come from team and
// role data, not from here). Each field is ServiceNow's matching
// x_wso2_customer_0.escalation.* system property, copied as it is: the two
// list properties are comma-separated there and a []string here, the rest
// are one address each. Every field is optional: empty means "no recipients
// from this slot", never a request failure.
type EscalationNotificationConfig struct {
	// EL1AmericasTLEmails is escalation.el1.americas_tl_emails.
	EL1AmericasTLEmails []string
	// EL2AmericasTUEmails is escalation.el2.americas_tu_emails.
	EL2AmericasTUEmails []string
	// EL2ProductServiceEmail / EL2ProductIdentityServerEmail /
	// EL2ProductDefaultEmail are escalation.el2.product_email.service /
	// .identity_server / .default.
	EL2ProductServiceEmail        string
	EL2ProductIdentityServerEmail string
	EL2ProductDefaultEmail        string
}

// The EL3-EL5 recipient sources: a team position and two app roles.
const (
	escalationCREHeadPosition = "cre_head"
	escalationEL4Role         = "case_escalation_el4"
	escalationEL5Role         = "case_escalation_el5"
)

// recipientDirectory resolves the non-per-case recipients to "user".id
// values: configured addresses, team positions and app roles. An interface,
// not direct query calls, so tests can substitute an in-memory fixture (see
// escalation_repo_test.go's fakeRecipientDirectory). Every method takes a
// rowsQuerier (case_repo.go, satisfied by both *pgxpool.Pool and pgx.Tx) so
// CreateEscalation can run it on the SAME open tx that already holds the
// case row's FOR UPDATE lock -- see resolveEscalationRecipients's own doc
// comment for why that matters. None of team_member, role or user_role is
// under row-level security, so a customer-triggered escalation resolves the
// same people an internal one does.
type recipientDirectory interface {
	// UserIDsByEmails returns one "user".id per address that matches a user,
	// in the order given; an address with no user is skipped, not an error.
	UserIDsByEmails(ctx context.Context, q rowsQuerier, emails []string) ([]string, error)
	// TeamPositionHolders returns every user holding position
	// (team_member.role) in any team, earliest membership first.
	TeamPositionHolders(ctx context.Context, q rowsQuerier, position string) ([]string, error)
	// AppRoleHolders returns every user granted the app role (role.name,
	// through user_role), earliest grant first.
	AppRoleHolders(ctx context.Context, q rowsQuerier, role string) ([]string, error)
}

// dbRecipientDirectory is recipientDirectory's real implementation --
// stateless: every call receives its querier explicitly.
type dbRecipientDirectory struct{}

// UserIDsByEmails implements recipientDirectory. ServiceNow looks each
// address up by sys_user.user_name (which holds the email there) with
// setLimit(1); this matches user_name or email, case-insensitively, and keeps
// one user per address, preferring a user_name match.
func (r *dbRecipientDirectory) UserIDsByEmails(ctx context.Context, q rowsQuerier, emails []string) ([]string, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	return queryUserIDs(ctx, q, "query escalation recipients by email", `
		SELECT DISTINCT ON (e.ord) u.id::TEXT
		FROM unnest($1::text[]) WITH ORDINALITY AS e(addr, ord)
		JOIN "user" u ON LOWER(u.user_name) = LOWER(e.addr) OR LOWER(u.email) = LOWER(e.addr)
		ORDER BY e.ord, (LOWER(u.user_name) = LOWER(e.addr)) DESC, u.id`, emails)
}

// TeamPositionHolders implements recipientDirectory.
func (r *dbRecipientDirectory) TeamPositionHolders(ctx context.Context, q rowsQuerier, position string) ([]string, error) {
	return queryUserIDs(ctx, q, "query escalation recipients by team position", `
		SELECT tm.user_id::TEXT
		FROM team_member tm
		WHERE tm.role = $1
		GROUP BY tm.user_id
		ORDER BY MIN(tm.created_on), tm.user_id`, position)
}

// AppRoleHolders implements recipientDirectory.
func (r *dbRecipientDirectory) AppRoleHolders(ctx context.Context, q rowsQuerier, role string) ([]string, error) {
	return queryUserIDs(ctx, q, "query escalation recipients by role", `
		SELECT ur.user_id::TEXT
		FROM user_role ur
		JOIN role r ON r.id = ur.role_id
		WHERE r.name = $1
		GROUP BY ur.user_id
		ORDER BY MIN(ur.created_on), ur.user_id`, role)
}

// queryUserIDs runs a query returning one user id per row.
func queryUserIDs(ctx context.Context, q rowsQuerier, what, sql string, arg any) ([]string, error) {
	rows, err := q.Query(ctx, sql, arg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", what, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: iterate: %w", what, err)
	}
	return ids, nil
}

type escalationRepo struct {
	db        *Scoped
	notifyCfg EscalationNotificationConfig
	dir       recipientDirectory
}

// NewEscalationRepository constructs an EscalationRepository backed by the
// given Scoped connection -- never a raw *pgxpool.Pool, so every query this
// repository issues carries the caller's identity for case_escalation's RLS
// policy to read.
func NewEscalationRepository(db *Scoped, notifyCfg EscalationNotificationConfig) EscalationRepository {
	return &escalationRepo{db: db, notifyCfg: notifyCfg, dir: &dbRecipientDirectory{}}
}

// escalationLevelToEnum/escalationLevelFromEnum convert between
// SearchEscalationsFilters.CurrentLevels' plain ints (0..5, the same
// convention CaseView.EscalationLevel's own doc comment uses) and
// case_escalation_level_enum's 'EL0'..'EL5' labels.
func escalationLevelToEnum(level int) string {
	return "EL" + strconv.Itoa(level)
}

// escalationChoiceItem builds a domain.ChoiceListItem for a case_escalation_level_enum
// value. Unlike the ServiceNow data source, Postgres has no human-readable label for an
// escalation level anywhere in this schema -- ID and Label are both the plain "0".."5"
// id (case_escalation_level_enum's 'EL' prefix stripped) rather than inventing display
// text this data source has no source for.
func escalationChoiceItem(enumValue string) domain.ChoiceListItem {
	id := strings.TrimPrefix(enumValue, "EL")
	return domain.ChoiceListItem{ID: id, Label: id}
}

const escalationSelectColumns = `
	ce.id, ce.work_item_id, wi.number, wi.subject, wi.wso2_id,
	ce.current_level::TEXT, ce.previous_level::TEXT,
	ce.created_by, ce.created_on, ce.updated_on, ce.reason`

const escalationFromJoins = `
	FROM case_escalation ce
	JOIN work_item wi ON wi.id = ce.work_item_id`

func scanEscalation(row interface{ Scan(...any) error }) (domain.Escalation, error) {
	var (
		e                               domain.Escalation
		caseID, caseNumber, caseSubject string
		wso2ID                          *string
		currentLevel, previousLevel     *string
		createdOn, updatedOn            time.Time
	)
	err := row.Scan(
		&e.ID, &caseID, &caseNumber, &caseSubject, &wso2ID,
		&currentLevel, &previousLevel,
		&e.CreatedBy, &createdOn, &updatedOn, &e.Reason,
	)
	if err != nil {
		return domain.Escalation{}, err
	}
	e.Case = domain.ReferenceTableItem{ID: caseID, Name: caseSubject, Number: &caseNumber, InternalID: stringPtrOrNil(wso2ID)}
	if currentLevel != nil {
		e.CurrentLevel = escalationChoiceItem(*currentLevel)
	}
	if previousLevel != nil {
		e.PreviousLevel = escalationChoiceItem(*previousLevel)
	}
	e.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	e.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	return e, nil
}

// stringPtrOrNil returns nil for a nil or blank *string, otherwise itself --
// wso2_id can be NULL or ” (see case_repo.go's own note on this), and
// ReferenceTableItem.InternalID must stay nil rather than render "".
func stringPtrOrNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// getEscalationNotifiedUsers batch-fetches the notification list for every
// id in escalationIDs, avoiding one query per escalation. q is a
// rowsQuerier (case_repo.go) rather than always r.db, so CreateEscalation
// can run this against the open tx before commit -- see that method's own
// call site for why.
func (r *escalationRepo) getEscalationNotifiedUsers(ctx context.Context, q rowsQuerier, escalationIDs []string) (map[string][]domain.EscalationNotifiedUser, error) {
	out := map[string][]domain.EscalationNotifiedUser{}
	if len(escalationIDs) == 0 {
		return out, nil
	}

	rows, err := q.Query(ctx, `
		SELECT cenl.case_escalation_id, u.id, u.user_name, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email
		FROM case_escalation_notification_list cenl
		JOIN "user" u ON u.id = cenl.user_id
		WHERE cenl.case_escalation_id = ANY($1::uuid[])
		ORDER BY cenl.id`, escalationIDs)
	if err != nil {
		return nil, fmt.Errorf("list escalation notified users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var escalationID, userID, userName string
		var name, email *string
		if err := rows.Scan(&escalationID, &userID, &userName, &name, &email); err != nil {
			return nil, fmt.Errorf("scan escalation notified user: %w", err)
		}
		out[escalationID] = append(out[escalationID], domain.EscalationNotifiedUser{ID: userID, UserName: userName, Name: name, Email: email})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate escalation notified users: %w", err)
	}
	return out, nil
}

// SearchEscalations implements EscalationRepository.
func (r *escalationRepo) SearchEscalations(ctx context.Context, caseIDs []string, currentLevels []int, sortField, sortOrder string, limit, offset int) ([]domain.Escalation, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(caseIDs) > 0 {
		args = append(args, caseIDs)
		where += fmt.Sprintf(" AND ce.work_item_id = ANY($%d::uuid[])", len(args))
	}
	if len(currentLevels) > 0 {
		levels := make([]string, len(currentLevels))
		for i, l := range currentLevels {
			levels[i] = escalationLevelToEnum(l)
		}
		args = append(args, levels)
		// ::text[] before ::case_escalation_level_enum[]: this repository
		// never registers case_escalation_level_enum/_case_escalation_level_enum
		// with pgx, so binding a []string directly to the enum array type has
		// no encode plan -- same fix as time_card_repo.go's state filter.
		where += fmt.Sprintf(" AND ce.current_level = ANY($%d::text[]::case_escalation_level_enum[])", len(args))
	}

	sortCol := "ce.created_on"
	if sortField == "updatedOn" {
		sortCol = "ce.updated_on"
	}
	sortDir := "DESC"
	if sortOrder == "asc" {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + escalationFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY %s %s, ce.id LIMIT $%d OFFSET $%d",
		escalationSelectColumns, escalationFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var escalations []domain.Escalation

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count escalations: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query escalations: %w", err)
		}
		defer rows.Close()

		out := make([]domain.Escalation, 0, limit)
		for rows.Next() {
			e, err := scanEscalation(rows)
			if err != nil {
				return fmt.Errorf("scan escalation: %w", err)
			}
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate escalations: %w", err)
		}
		escalations = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	ids := make([]string, len(escalations))
	for i, e := range escalations {
		ids[i] = e.ID
	}
	notifiedByEscalation, err := r.getEscalationNotifiedUsers(ctx, r.db, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range escalations {
		// openapi.yaml declares notificationSentTo as a required, non-nullable
		// array -- a map miss (no notification list for this escalation)
		// returns a nil slice, which json.Encode would render as null.
		notified := notifiedByEscalation[escalations[i].ID]
		if notified == nil {
			notified = []domain.EscalationNotifiedUser{}
		}
		escalations[i].NotificationSentTo = notified
	}

	return escalations, total, nil
}

// maxEscalationLevel is case_escalation_level_enum's ceiling (EL5) --
// ESCALATE from it is refused.
const maxEscalationLevel = 5

// escalationLevelInt parses a nullable current_level/previous_level column
// ("EL0".."EL5", or NULL for a case never escalated) into a plain 0..5 int.
// NULL is treated as EL0 -- "case".current_escalation_level/is_escalated
// have no NOT NULL constraint or default (migration 0023), and a case
// that's never been escalated is exactly the EL0 state.
func escalationLevelInt(raw *string) int {
	if raw == nil {
		return 0
	}
	n, err := strconv.Atoi(caseEscalationLevelFromEnum(*raw))
	if err != nil {
		return 0
	}
	return n
}

// nextEscalationLevel computes the resulting level for action given the
// case's current level (previous int, already normalized via
// escalationLevelInt) -- see EscalationRepository.CreateEscalation's own doc
// comment for the full rule: ESCALATE goes up one, DEESCALATE goes to EL0,
// and either one with nowhere to go is SN's 409.
func nextEscalationLevel(action domain.EscalationAction, previous int) (int, error) {
	if action == domain.EscalationActionDeescalate {
		if previous == 0 {
			return 0, &apierror.ConflictError{Msg: "case is not currently escalated (already at EL0); nothing to de-escalate"}
		}
		return 0, nil
	}
	// ESCALATE, and any already-validated-upstream default.
	if previous >= maxEscalationLevel {
		return 0, &apierror.ConflictError{Msg: "case is already at the maximum escalation level (EL5); no further escalation is possible"}
	}
	return previous + 1, nil
}

// notificationLevel is the level whose cumulative EL1..N list an escalation
// notifies: the level reached for ESCALATE, the level left for DEESCALATE.
func notificationLevel(action domain.EscalationAction, previous, next int) int {
	if action == domain.EscalationActionDeescalate {
		return previous
	}
	return next
}

// escalationCaseContext is everything CreateEscalation needs about the case
// beyond its current/previous level, gathered in the same locked read so the
// recipient computation below is consistent with the level transition it's
// reacting to.
type escalationCaseContext struct {
	number, subject  string
	wso2ID           *string
	accountManagerID *string
	technicalOwnerID *string
	csmID            *string
	// teamLeadIDs are the CRE team's 'lead' members, oldest membership first.
	teamLeadIDs     []string
	productCategory *string
	productCode     *string
	productName     *string
}

// wso2ISProductCode / wso2ISProductName are how ServiceNow's
// EscalationNotificationUtils recognises WSO2 Identity Server for EL2
// routing (WSO2_IS_PRODUCT_CODE / WSO2_IS_PRODUCT_NAME): either one matches.
const (
	wso2ISProductCode = "wso2is"
	wso2ISProductName = "WSO2 Identity Server"
)

// el2ProductEmail is SN's _resolveProductBasedUser: which one EL2 product
// contact a case gets. A case with no product, and any product that is
// neither a service nor Identity Server, gets the default.
func (cc escalationCaseContext) el2ProductEmail(cfg EscalationNotificationConfig) string {
	if cc.productCategory == nil {
		return cfg.EL2ProductDefaultEmail
	}
	if *cc.productCategory == "SERVICE" {
		return cfg.EL2ProductServiceEmail
	}
	if *cc.productCategory == "SOFTWARE" &&
		((cc.productCode != nil && *cc.productCode == wso2ISProductCode) ||
			(cc.productName != nil && *cc.productName == wso2ISProductName)) {
		return cfg.EL2ProductIdentityServerEmail
	}
	return cfg.EL2ProductDefaultEmail
}

// resolveEscalationRecipients implements the cumulative EL1..EL5 rule
// described on EscalationRepository.CreateEscalation's own doc comment,
// returning "user".id values deduped in first-seen order (SN's seenIds) for
// EL1..newLevel, where newLevel is notificationLevel's pick.
//
// q is CreateEscalation's own open tx, not r's pool -- CreateEscalation
// holds one pool connection for that tx, with a FOR UPDATE lock on the case
// row, for its whole duration. Acquiring a SECOND pool connection here would
// mean a saturated pool blocks every concurrent escalation on pool.Acquire
// while it still holds its own tx connection and the case-row lock. Running
// the read on q=tx needs no second connection; the reads are read-only, so
// this adds no extra locks.
func (r *escalationRepo) resolveEscalationRecipients(ctx context.Context, q rowsQuerier, newLevel int, cc escalationCaseContext) ([]string, error) {
	seen := map[string]bool{}
	var ids []string
	add := func(id *string) {
		if id != nil && *id != "" && !seen[*id] {
			seen[*id] = true
			ids = append(ids, *id)
		}
	}
	addEmails := func(emails ...string) error {
		var clean []string
		for _, e := range emails {
			if e = strings.TrimSpace(e); e != "" {
				clean = append(clean, e)
			}
		}
		if len(clean) == 0 {
			return nil
		}
		found, err := r.dir.UserIDsByEmails(ctx, q, clean)
		if err != nil {
			return fmt.Errorf("resolve escalation recipients: %w", err)
		}
		for i := range found {
			add(&found[i])
		}
		return nil
	}

	addFrom := func(found []string, err error) error {
		if err != nil {
			return fmt.Errorf("resolve escalation recipients: %w", err)
		}
		for i := range found {
			add(&found[i])
		}
		return nil
	}

	if newLevel >= 1 {
		for i := range cc.teamLeadIDs {
			add(&cc.teamLeadIDs[i])
		}
		if err := addEmails(r.notifyCfg.EL1AmericasTLEmails...); err != nil {
			return nil, err
		}
		add(cc.accountManagerID)
		add(cc.technicalOwnerID)
	}
	if newLevel >= 2 {
		if err := addEmails(r.notifyCfg.EL2AmericasTUEmails...); err != nil {
			return nil, err
		}
		if err := addEmails(cc.el2ProductEmail(r.notifyCfg)); err != nil {
			return nil, err
		}
	}
	if newLevel >= 3 {
		if err := addFrom(r.dir.TeamPositionHolders(ctx, q, escalationCREHeadPosition)); err != nil {
			return nil, err
		}
		add(cc.csmID)
	}
	if newLevel >= 4 {
		if err := addFrom(r.dir.AppRoleHolders(ctx, q, escalationEL4Role)); err != nil {
			return nil, err
		}
	}
	if newLevel >= 5 {
		if err := addFrom(r.dir.AppRoleHolders(ctx, q, escalationEL5Role)); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// CreateEscalation implements EscalationRepository.
func (r *escalationRepo) CreateEscalation(ctx context.Context, caseID string, action domain.EscalationAction, reason *string, actorEmail string) (domain.CreatedEscalation, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.CreatedEscalation, error) {
		return r.createEscalationTx(ctx, tx, caseID, action, reason, actorEmail)
	})
}

// createEscalationTx is CreateEscalation's body, extracted so it can run
// inside r.db.InTx's closure (Scoped.InTx pulls caller identity from ctx
// and sets it once for the whole transaction).
func (r *escalationRepo) createEscalationTx(ctx context.Context, tx pgx.Tx, caseID string, action domain.EscalationAction, reason *string, actorEmail string) (domain.CreatedEscalation, error) {
	var (
		currentLevel *string
		cc           escalationCaseContext
	)
	err := tx.QueryRow(ctx, `
		SELECT c.current_escalation_level::TEXT,
		       wi.number, wi.subject, wi.wso2_id,
		       a.account_manager_id, a.technical_owner_id, a.customer_success_manager_id,
		       ARRAY(SELECT tm.user_id::TEXT FROM team_member tm
		             WHERE tm.team_id = a.cre_team_id AND tm.role = 'lead'
		             ORDER BY tm.created_on, tm.user_id),
		       prod.category::TEXT, prod.code, prod.name
		FROM "case" c
		JOIN work_item wi ON wi.id = c.id
		LEFT JOIN account a ON a.id = wi.account_id
		LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		LEFT JOIN product prod ON prod.id = dp.product_id
		WHERE c.id = $1
		FOR UPDATE OF c`, caseID,
	).Scan(
		&currentLevel, &cc.number, &cc.subject, &cc.wso2ID,
		&cc.accountManagerID, &cc.technicalOwnerID, &cc.csmID, &cc.teamLeadIDs,
		&cc.productCategory, &cc.productCode, &cc.productName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreatedEscalation{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CreatedEscalation{}, fmt.Errorf("create escalation: lock case: %w", err)
	}

	previousLevelInt := escalationLevelInt(currentLevel)
	newLevelInt, err := nextEscalationLevel(action, previousLevelInt)
	if err != nil {
		return domain.CreatedEscalation{}, err
	}
	isEscalated := newLevelInt >= 1

	recipientIDs, err := r.resolveEscalationRecipients(ctx, tx, notificationLevel(action, previousLevelInt, newLevelInt), cc)
	if err != nil {
		return domain.CreatedEscalation{}, err
	}

	var escalationID string
	var createdOn time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO case_escalation (id, created_on, updated_on, created_by, updated_by, work_item_id, current_level, previous_level, reason)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, $3::case_escalation_level_enum, $4::case_escalation_level_enum, $5)
		RETURNING id, created_on`,
		actorEmail, caseID, escalationLevelToEnum(newLevelInt), escalationLevelToEnum(previousLevelInt), reason,
	).Scan(&escalationID, &createdOn)
	if err != nil {
		return domain.CreatedEscalation{}, fmt.Errorf("create escalation: insert case_escalation: %w", err)
	}

	if len(recipientIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO case_escalation_notification_list (id, case_escalation_id, user_id)
			SELECT gen_random_uuid(), $1, u FROM unnest($2::uuid[]) AS u`,
			escalationID, recipientIDs,
		); err != nil {
			return domain.CreatedEscalation{}, fmt.Errorf("create escalation: insert notification list: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE "case" SET current_escalation_level = $2::case_escalation_level_enum, is_escalated = $3 WHERE id = $1`,
		caseID, escalationLevelToEnum(newLevelInt), isEscalated,
	); err != nil {
		return domain.CreatedEscalation{}, fmt.Errorf("create escalation: update case: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1`,
		caseID, actorEmail,
	); err != nil {
		return domain.CreatedEscalation{}, fmt.Errorf("create escalation: update work_item: %w", err)
	}

	// Read back INSIDE the still-open tx, before commit -- not after. The
	// response needs richer fields (userName, name, email) than
	// recipientIDs alone carries (userName is a required field on
	// EscalationNotifiedUser per openapi.yaml, so it can't be synthesized
	// from just a user id), and reuses getEscalationNotifiedUsers's own
	// join rather than re-deriving it. Reading uncommitted rows this tx
	// itself just inserted is fine (a transaction sees its own writes).
	// Doing this AFTER commit instead (the original approach) meant a
	// failure here -- transient DB issue, pool timeout -- surfaced as a 5xx
	// for a write that had already succeeded, and a client retry would then
	// create a second, orphaned case_escalation row. Inside the tx, the
	// same failure safely rolls back the whole escalation instead.
	notifiedByEscalation, err := r.getEscalationNotifiedUsers(ctx, tx, []string{escalationID})
	if err != nil {
		return domain.CreatedEscalation{}, err
	}
	notified := notifiedByEscalation[escalationID]
	if notified == nil {
		notified = []domain.EscalationNotifiedUser{}
	}

	return domain.CreatedEscalation{
		ID:                 escalationID,
		Case:               domain.ReferenceTableItem{ID: caseID, Name: cc.subject, Number: &cc.number, InternalID: stringPtrOrNil(cc.wso2ID)},
		CurrentLevel:       escalationChoiceItem(escalationLevelToEnum(newLevelInt)),
		PreviousLevel:      escalationChoiceItem(escalationLevelToEnum(previousLevelInt)),
		CreatedBy:          actorEmail,
		CreatedOn:          createdOn.UTC().Format(time.RFC3339),
		Reason:             reason,
		NotificationSentTo: notified,
	}, nil
}

// CaseTeamLeads implements EscalationRepository.
func (r *escalationRepo) CaseTeamLeads(ctx context.Context, caseID string) ([]domain.EscalationNotifiedUser, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, u.user_name, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email
		FROM work_item wi
		JOIN account a ON a.id = wi.account_id
		JOIN team_member tm ON tm.team_id = a.cre_team_id AND tm.role = 'lead'
		JOIN "user" u ON u.id = tm.user_id
		WHERE wi.id = $1
		ORDER BY tm.created_on, u.id`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list case team leads: %w", err)
	}
	defer rows.Close()
	leads := []domain.EscalationNotifiedUser{}
	for rows.Next() {
		var l domain.EscalationNotifiedUser
		if err := rows.Scan(&l.ID, &l.UserName, &l.Name, &l.Email); err != nil {
			return nil, fmt.Errorf("scan case team lead: %w", err)
		}
		leads = append(leads, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case team leads: %w", err)
	}
	return leads, nil
}
