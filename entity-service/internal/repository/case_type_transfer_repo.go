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
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CaseTypeTransferRepository moves a case between the four types it can be
// transferred between. It is a separate interface rather than a method on
// CaseRepository so the many CaseRepository fakes need not grow a method for a
// capability only the Postgres-backed case service wires up (see
// service.WithCaseTypeTransfer).
type CaseTypeTransferRepository interface {
	// TransferCaseType changes a case's type in ONE transaction: the extension
	// row of the old type is replaced by one of the new type, work_item.type is
	// moved, and, when the LOW/S4 boundary is crossed, the case's time cards
	// are re-marked billable or not (the same rule a severity change applies).
	//
	// remote, when non-nil, is called inside that transaction after every
	// Postgres statement has succeeded and before it commits -- the dual-write
	// service uses it to perform the ServiceNow transfer. An error from it rolls
	// the whole transfer back, so ServiceNow rejecting a transfer (a missing
	// catalog answer, a type it will not accept) leaves Postgres untouched, and
	// a Postgres failure (a constraint, a missing row) is found before
	// ServiceNow is ever asked. Only a failed COMMIT after a successful remote
	// call can leave the two stores apart, which is logged by the caller.
	TransferCaseType(ctx context.Context, plan CaseTypeTransfer, remote CaseTypeTransferRemote) (CaseTypeTransferResult, error)
}

// CaseTypeTransfer is what to transfer a case into.
type CaseTypeTransfer struct {
	CaseID string
	// TargetType is one of "case", "engagement", "service_request",
	// "security_report_analysis" (already normalized and validated by the
	// service).
	TargetType string
	// Severity and IssueType are required for TargetType "case" and ignored
	// otherwise. Severity decides Incident (S0-S3) vs Query (S4) in
	// ServiceNow; Postgres keeps both in the "case" table, told apart by it.
	Severity  *domain.CaseSeverity
	IssueType *domain.CaseIssueType
	// EngagementType and EngagementPaymentType are required for TargetType
	// "engagement" and ignored otherwise.
	EngagementType        *domain.EngagementType
	EngagementPaymentType *domain.EngagementPaymentType
	// ActorEmail is the calling user, resolved by the service from the caller's
	// token. It is optional: a caller that cannot be resolved still transfers, it
	// is simply not named on work_item.updated_by.
	ActorEmail string
}

// CaseTypeTransferBefore is what the case looked like immediately before the
// transfer, handed to the remote step.
type CaseTypeTransferBefore struct {
	PreviousType     string
	PreviousSeverity *domain.CaseSeverity
}

// CaseTypeTransferRemoteResult is what the remote step reports back.
type CaseTypeTransferRemoteResult struct {
	// State is the case state the remote system reports after the transfer.
	// When set it is written to the new extension row, so Postgres shows what
	// the system of record shows rather than what it carried over.
	State *domain.CaseState
}

// CaseTypeTransferRemote is the remote step; see TransferCaseType.
type CaseTypeTransferRemote func(ctx context.Context, before CaseTypeTransferBefore) (*CaseTypeTransferRemoteResult, error)

// CaseTypeTransferResult describes the committed transfer.
type CaseTypeTransferResult struct {
	PreviousType     string
	PreviousSeverity *domain.CaseSeverity
	Type             string
	// Severity is set only when the new type is "case".
	Severity  *domain.CaseSeverity
	State     *domain.CaseState
	WorkState *domain.CaseWorkState
	UpdatedOn time.Time
	ProjectID string
}

// caseTypeToWorkItemType maps a transfer target to work_item.type.
var caseTypeToWorkItemType = map[string]string{
	"case":                     "CASE",
	"engagement":               "ENGAGEMENT",
	"service_request":          "SERVICE_REQUEST",
	"security_report_analysis": "SECURITY_REPORT_ANALYSIS",
}

// workItemTypeToCaseType is its inverse, covering announcements too so a
// transfer OUT of one can be refused by name rather than by omission.
var workItemTypeToCaseType = map[string]string{
	"CASE":                     "case",
	"ENGAGEMENT":               "engagement",
	"SERVICE_REQUEST":          "service_request",
	"SECURITY_REPORT_ANALYSIS": "security_report_analysis",
	"ANNOUNCEMENT":             "announcement",
}

// caseTypeExtension names, per transfer target, the extension table and the
// enum types whose labels the carried-over values are cast to. Every value
// here is a constant, never request text, so it is safe to splice into SQL.
type caseTypeExtension struct {
	table     string
	stateEnum string
	causeEnum string
}

var caseTypeExtensions = map[string]caseTypeExtension{
	"case":                     {table: `"case"`, stateEnum: "case_state_enum", causeEnum: "case_cause_enum"},
	"engagement":               {table: "engagement", stateEnum: "engagement_state_enum", causeEnum: "engagement_cause_enum"},
	"service_request":          {table: "service_request", stateEnum: "service_request_state_enum", causeEnum: "service_request_cause_enum"},
	"security_report_analysis": {table: "security_report_analysis", stateEnum: "security_report_analysis_state_enum", causeEnum: "security_report_analysis_cause_enum"},
}

// carriedCaseFields is what every case-like extension row has in common, i.e.
// what survives a transfer. Everything type-specific (severity and issue type
// of a "case", the type and payment type of an engagement, the catalog of a
// service request) is dropped or supplied afresh.
type carriedCaseFields struct {
	state              *string
	closeNotes         *string
	cause              *string
	closedByUserID     *string
	closedOn           *time.Time
	resolvedOn         *time.Time
	autoclosureStep    *string
	autoclosureStateOn *time.Time
	workState          *string
	resolutionCode     *string
	severity           *string
}

// readCarriedCaseFieldsSQL reads the common columns of the extension row of
// each type, locking it. severity exists on "case" only. All five tables carry
// work_state and resolution_code since migration 0184.
func readCarriedCaseFieldsSQL(previousType string) string {
	ext := caseTypeExtensions[previousType]
	severity := "NULL::TEXT"
	if previousType == "case" {
		severity = "severity::TEXT"
	}
	return fmt.Sprintf(`SELECT state::TEXT, close_notes, cause::TEXT, closed_by_user_id::TEXT, closed_on, resolved_on,
	       autoclosure_step, autoclosure_state_on, work_state::TEXT, resolution_code::TEXT, %s
	  FROM %s WHERE id = $1::uuid FOR UPDATE`, severity, ext.table)
}

// insertTransferredCaseSQL builds the INSERT of the new extension row. The
// first eleven parameters are the same for every target ($1 id, $2 state,
// $3 close notes, $4 cause, $5 closed-by user, $6 closed on, $7 resolved on,
// $8 auto-closure step, $9 auto-closure time, $10 work state, $11 resolution
// code); a target's own columns follow.
func insertTransferredCaseSQL(target string) string {
	ext := caseTypeExtensions[target]
	common := fmt.Sprintf(`$1::uuid, NULLIF($2,'')::%s, $3, NULLIF($4,'')::%s, NULLIF($5,'')::uuid, $6, $7, $8, $9,
	        NULLIF($10,'')::case_work_state_enum, NULLIF($11,'')::case_resolution_code_enum`, ext.stateEnum, ext.causeEnum)
	commonCols := `id, state, close_notes, cause, closed_by_user_id, closed_on, resolved_on,
	        autoclosure_step, autoclosure_state_on, work_state, resolution_code`
	switch target {
	case "case":
		return fmt.Sprintf(`INSERT INTO %s (%s, severity, issue_type)
	VALUES (%s, $12::case_severity_enum, $13::case_issue_type_enum)`, ext.table, commonCols, common)
	case "engagement":
		return fmt.Sprintf(`INSERT INTO %s (%s, type, payment_type)
	VALUES (%s, $12::engagement_type_enum, $13::engagement_payment_type_enum)`, ext.table, commonCols, common)
	default:
		return fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, ext.table, commonCols, common)
	}
}

// relatedCaseListLimit is how many case numbers a refusal names before it says
// "and others".
const relatedCaseListLimit = 5

// relatedCaseKeyTargetsCaseSQL reports whether "case".related_case_id still has its
// original foreign key into "case" (i.e. migration 0211 has not been applied).
const relatedCaseKeyTargetsCaseSQL = `
	SELECT EXISTS (
	  SELECT 1 FROM pg_constraint con
	   WHERE con.conrelid = to_regclass('"case"') AND con.contype = 'f'
	     AND con.confrelid = to_regclass('"case"')
	     AND con.conkey = ARRAY[(SELECT attnum FROM pg_attribute
	                              WHERE attrelid = to_regclass('"case"') AND attname = 'related_case_id' AND NOT attisdropped)])`

// relatedCasesBlockMessage words the refusal for a case that other cases name as
// their related case while the database still ties that link to "case" rows. related
// holds up to relatedCaseListLimit+1 numbers: one more than is named, which is how
// "and others" is known.
func relatedCasesBlockMessage(related []string) string {
	named := related
	more := ""
	if len(related) > relatedCaseListLimit {
		named = related[:relatedCaseListLimit]
		more = " and others"
	}
	return "this case is the related case of " + strings.Join(named, ", ") + more +
		", and the database still ties that link to cases only, so it cannot be moved to another type until the related case migration (0211) has been applied"
}

// TransferCaseType implements CaseTypeTransferRepository.
func (r *caseRepo) TransferCaseType(ctx context.Context, plan CaseTypeTransfer, remote CaseTypeTransferRemote) (CaseTypeTransferResult, error) {
	targetWorkItemType, ok := caseTypeToWorkItemType[plan.TargetType]
	if !ok {
		return CaseTypeTransferResult{}, &apierror.ValidationError{Msg: "type " + plan.TargetType + " is not a case type a case can be transferred to"}
	}
	target := caseTypeExtensions[plan.TargetType]

	var targetSeverity, targetIssueType, targetEngagementType, targetPaymentType string
	switch plan.TargetType {
	case "case":
		if plan.Severity == nil || plan.IssueType == nil {
			return CaseTypeTransferResult{}, &apierror.ValidationError{Msg: "severity and issueType are required when type is \"case\""}
		}
		sev, ok := caseSeverityToEnum[*plan.Severity]
		if !ok {
			return CaseTypeTransferResult{}, &apierror.ValidationError{Msg: "severity contains invalid value: " + string(*plan.Severity)}
		}
		targetSeverity = sev
		targetIssueType = strings.ToUpper(string(*plan.IssueType))
	case "engagement":
		if plan.EngagementType == nil || plan.EngagementPaymentType == nil {
			return CaseTypeTransferResult{}, &apierror.ValidationError{Msg: "engagementType and engagementPaymentType are required when type is \"engagement\""}
		}
		targetEngagementType = strings.ToUpper(string(*plan.EngagementType))
		targetPaymentType = strings.ToUpper(string(*plan.EngagementPaymentType))
	}

	var result CaseTypeTransferResult
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		// 1. Lock the work item and read its current type. FOR NO KEY UPDATE is what
		//    a plain UPDATE of work_item takes: it excludes a concurrent update of the
		//    same case without blocking the foreign-key probes other tables make.
		var currentType string
		var projectID *string
		err := tx.QueryRow(ctx,
			`SELECT type::TEXT, project_id::TEXT FROM work_item
			  WHERE id = $1::uuid AND type = ANY(`+caseLikeWorkItemTypes+`) FOR NO KEY UPDATE`,
			plan.CaseID).Scan(&currentType, &projectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "case not found"}
		}
		if err != nil {
			return fmt.Errorf("transfer case type: read work item: %w", err)
		}
		previousType := workItemTypeToCaseType[currentType]
		if previousType == "announcement" {
			return &apierror.ValidationError{Msg: "an announcement is system-managed and cannot be transferred to another type"}
		}
		if previousType == plan.TargetType {
			return &apierror.ValidationError{Msg: "the case is already of type " + plan.TargetType}
		}

		// 1b. Other tickets may name this one as their related case. They stay what they
		//     are and stay related to it: only this ticket is converted. That needs the
		//     link to point at work_item (migration 0211). While it still points at
		//     "case", replacing this case's "case" row would clear the link on every
		//     one of them (ON DELETE SET NULL) -- a change to tickets this conversion
		//     does not own -- so until the migration is applied it is refused, naming
		//     them, before anything (ServiceNow included) is changed.
		if previousType == "case" {
			var oldKey bool
			if err := tx.QueryRow(ctx, relatedCaseKeyTargetsCaseSQL).Scan(&oldKey); err != nil {
				return fmt.Errorf("transfer case type: inspect the related case key: %w", err)
			}
			if oldKey {
				rows, err := tx.Query(ctx,
					`SELECT wi.number FROM "case" c JOIN work_item wi ON wi.id = c.id
					  WHERE c.related_case_id = $1::uuid AND c.id <> $1::uuid
					  ORDER BY wi.number LIMIT $2`, plan.CaseID, relatedCaseListLimit+1)
				if err != nil {
					return fmt.Errorf("transfer case type: look for cases related to this one: %w", err)
				}
				var related []string
				for rows.Next() {
					var number string
					if err := rows.Scan(&number); err != nil {
						rows.Close()
						return fmt.Errorf("transfer case type: read a related case: %w", err)
					}
					related = append(related, number)
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return fmt.Errorf("transfer case type: look for cases related to this one: %w", err)
				}
				if len(related) > 0 {
					return &apierror.ConflictError{Msg: relatedCasesBlockMessage(related)}
				}
			}
		}

		// 2. Read (and lock) what the old extension row carries over.
		var carried carriedCaseFields
		err = tx.QueryRow(ctx, readCarriedCaseFieldsSQL(previousType), plan.CaseID).Scan(
			&carried.state, &carried.closeNotes, &carried.cause, &carried.closedByUserID,
			&carried.closedOn, &carried.resolvedOn, &carried.autoclosureStep, &carried.autoclosureStateOn,
			&carried.workState, &carried.resolutionCode, &carried.severity)
		if errors.Is(err, pgx.ErrNoRows) {
			// A work item whose extension row is missing (staging holds some) has no
			// state to carry; the transfer still gives it a row of the new type.
			carried = carriedCaseFields{}
		} else if err != nil {
			return fmt.Errorf("transfer case type: read %s row: %w", previousType, err)
		}
		var previousSeverity *domain.CaseSeverity
		if carried.severity != nil {
			if s, ok := caseSeverityFromEnum[*carried.severity]; ok {
				previousSeverity = &s
			}
		}

		// 3. Replace the extension row: delete the old, insert the new.
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1::uuid`, caseTypeExtensions[previousType].table), plan.CaseID); err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				// case_attachment.case_id references "case"(id) with no cascade (migration
				// 0106), so an attachment keeps its case row from being replaced.
				slog.ErrorContext(ctx, "transfer case type: the old extension row is still referenced",
					"caseId", plan.CaseID, "from", previousType, "constraint", pgErr.ConstraintName, "table", pgErr.TableName)
				return &apierror.ConflictError{Msg: "this case has attachments that still point at its current type, so it cannot be moved to another type until the attachment storage is updated; contact the platform team"}
			}
			return fmt.Errorf("transfer case type: delete %s row: %w", previousType, err)
		}
		insertArgs := []any{
			plan.CaseID, stringOrEmpty(carried.state), carried.closeNotes, stringOrEmpty(carried.cause),
			stringOrEmpty(carried.closedByUserID), carried.closedOn, carried.resolvedOn,
			carried.autoclosureStep, carried.autoclosureStateOn,
			stringOrEmpty(carried.workState), stringOrEmpty(carried.resolutionCode),
		}
		switch plan.TargetType {
		case "case":
			insertArgs = append(insertArgs, targetSeverity, targetIssueType)
		case "engagement":
			insertArgs = append(insertArgs, targetEngagementType, targetPaymentType)
		}
		if _, err := tx.Exec(ctx, insertTransferredCaseSQL(plan.TargetType), insertArgs...); err != nil {
			return fmt.Errorf("transfer case type: insert %s row: %w", plan.TargetType, err)
		}

		// 4. work_item.type, and who/when.
		var updatedOn time.Time
		if err := tx.QueryRow(ctx,
			`UPDATE work_item SET type = $2::work_item_type_enum, updated_on = NOW(),
			        updated_by = COALESCE(NULLIF($3, ''), updated_by)
			  WHERE id = $1::uuid RETURNING updated_on`,
			plan.CaseID, targetWorkItemType, plan.ActorEmail).Scan(&updatedOn); err != nil {
			return fmt.Errorf("transfer case type: update work item: %w", err)
		}

		// 5. LOW/S4 boundary. A "case" at S4 is a Query, whose time cards are
		//    billable unless the case carries a "patch" tag; every other severity,
		//    and every other type, is not. UpdateCase re-marks the time cards when a
		//    severity change crosses that line, and a transfer crosses it the same
		//    way when it moves an S4 case out of "case" or an S4 case in. It runs in
		//    a savepoint: a failing statement would otherwise abort the transfer.
		oldLow := previousSeverity != nil && *previousSeverity == domain.CaseSeverityLow
		newLow := plan.TargetType == "case" && plan.Severity != nil && *plan.Severity == domain.CaseSeverityLow
		if oldLow != newLow {
			if sp, spErr := tx.Begin(ctx); spErr == nil {
				if _, rcErr := recomputeTimeCardsBillable(ctx, sp, plan.CaseID, newLow); rcErr != nil {
					_ = sp.Rollback(ctx)
					slog.ErrorContext(ctx, "transfer case type: recompute time cards billable failed", "caseId", plan.CaseID, "error", rcErr)
				} else if cErr := sp.Commit(ctx); cErr != nil {
					return fmt.Errorf("transfer case type: release time card savepoint: %w", cErr)
				}
			} else {
				return fmt.Errorf("transfer case type: open time card savepoint: %w", spErr)
			}
		}

		// 6. The remote step. Everything above has succeeded; if it fails, all of it
		//    is rolled back with it.
		var remoteState *domain.CaseState
		if remote != nil {
			rr, err := remote(ctx, CaseTypeTransferBefore{PreviousType: previousType, PreviousSeverity: previousSeverity})
			if err != nil {
				return err
			}
			if rr != nil && rr.State != nil {
				remoteState = rr.State
				if _, err := tx.Exec(ctx,
					fmt.Sprintf(`UPDATE %s SET state = $2::%s WHERE id = $1::uuid`, target.table, target.stateEnum),
					plan.CaseID, strings.ToUpper(string(*rr.State))); err != nil {
					return fmt.Errorf("transfer case type: write remote state: %w", err)
				}
			}
		}

		result = CaseTypeTransferResult{
			PreviousType:     previousType,
			PreviousSeverity: previousSeverity,
			Type:             plan.TargetType,
			UpdatedOn:        updatedOn,
			ProjectID:        stringOrEmpty(projectID),
		}
		if plan.TargetType == "case" {
			result.Severity = plan.Severity
		}
		switch {
		case remoteState != nil:
			result.State = remoteState
		case carried.state != nil:
			st := domain.CaseState(strings.ToLower(*carried.state))
			result.State = &st
		}
		if carried.workState != nil {
			ws := domain.CaseWorkState(strings.ToLower(*carried.workState))
			result.WorkState = &ws
		}
		return nil
	})
	if err != nil {
		return CaseTypeTransferResult{}, err
	}
	return result, nil
}
