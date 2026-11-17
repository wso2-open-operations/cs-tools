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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file is what a change request raised from a standard change template
// (change_request.standard_change_template_version_id, migration 0252) does
// differently from any other:
//
//   - the fields the template specifies are frozen from creation on, in every
//     state: what is implemented, tested and backed out cannot deviate from the
//     CAB-approved template (checkStandardChangeTemplateFields);
//   - its type stays standard, and no other change request becomes a standard
//     change by a PATCH (checkStandardChangeTypeEdit);
//   - Request Approval needs the template still active and still at the version
//     the change was raised from (checkStandardChangeTemplateUsable);
//   - it is for a project of the template's customer, from creation on and in
//     every state (recordChangeRequestTemplate, checkStandardChangeProjectCustomer);
//     and since the Salesforce project sync (the source of truth for projects)
//     may move a project to another account afterwards, Request Approval checks
//     the project's account again (checkStandardChangeCustomerAtApproval) and the
//     detail says when they no longer match (standardChange.customerMismatch);
//   - a rollback, or a deviation from the implementation plan, flags the template
//     for review (flagStandardTemplateOnFailure).
//
// A change request with no template version is untouched by all of it.

// standardTemplateFields are the PATCH fields (approvedLockFields names) a
// template specifies.
var standardTemplateFields = map[string]bool{
	"title":                        true,
	"description":                  true,
	"justification":                true,
	"impactDescription":            true,
	"implementationPlan":           true,
	"rollbackPlan":                 true,
	"testPlan":                     true,
	"rollbackDurationText":         true,
	"impact":                       true,
	"category":                     true,
	"isPlanningVisibleToCustomers": true,
	"customerApprovalRequired":     true,
	"customerReviewRequired":       true,
}

func standardTemplateFieldsMsg(fields []string) string {
	return fmt.Sprintf("%s cannot be changed: this change request was raised from a standard change template, and what the template specifies cannot deviate from it; record a deviation from the implementation plan once it is implemented, or propose a change to the template",
		strings.Join(fields, ", "))
}

// checkStandardChangeTemplateFields refuses (400,
// CodeChangeRequestStandardTemplateField) a PATCH of a templated change request
// that changes a field the template specifies. A field resent with its stored
// value is the accepted no-op, as in the approved-content lock.
func checkStandardChangeTemplateFields(ctx context.Context, tx pgx.Tx, id string, snap changeRequestGateSnapshot, req domain.PatchChangeRequestRequest) error {
	if snap.templateVersionID == nil {
		return nil
	}
	var fields []approvedLockField
	for _, f := range approvedLockFields(req, true) {
		if standardTemplateFields[f.name] {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	query, args := approvedContentQuery(fields)
	args[0] = id
	var changed []string
	if err := tx.QueryRow(ctx, query, args...).Scan(&changed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "change request not found"}
		}
		return fmt.Errorf("patch change request: compare template fields: %w", err)
	}
	if len(changed) == 0 {
		return nil
	}
	return &apierror.ValidationError{Msg: standardTemplateFieldsMsg(changed), Code: apierror.CodeChangeRequestStandardTemplateField}
}

// checkStandardChangeTypeEdit keeps a templated change request standard.
// Resending the stored type is accepted. (The type is frozen after New anyway:
// checkChangeTypeEdit.) Keeping every other change request from becoming standard
// is the service's, while the catalogue is in use (refuseBecomingStandard).
func checkStandardChangeTypeEdit(snap changeRequestGateSnapshot, requested *domain.ChangeRequestType) error {
	if requested == nil {
		return nil
	}
	want := effectiveChangeModel(snap.model, requested)
	if want == snap.model {
		return nil
	}
	if snap.templateVersionID != nil {
		return &apierror.ValidationError{Code: apierror.CodeChangeRequestStandardTemplateField,
			Msg: "type cannot be changed: this change request was raised from a standard change template; cancel it and raise a new change request to use another type"}
	}
	return nil
}

// checkStandardChangeTemplateUsable refuses (409,
// CodeStandardChangeTemplateUnavailable) Request Approval of a templated change
// request whose template has been suspended or retired, or revised since the
// change was raised: the pre-authorisation is the template's current version
// only. The template row is held FOR SHARE so a suspension racing the request
// waits for it.
func checkStandardChangeTemplateUsable(ctx context.Context, tx pgx.Tx, snap changeRequestGateSnapshot) error {
	if snap.templateVersionID == nil || (snap.state != "" && snap.state != "NEW") {
		return nil
	}
	var state, current, name *string
	err := tx.QueryRow(ctx, `
		SELECT t.state, t.current_version_id::text, t.name
		  FROM standard_change_template_version v JOIN standard_change_template t ON t.id = v.template_id
		 WHERE v.id = $1::uuid
		 FOR SHARE OF t`, *snap.templateVersionID).Scan(&state, &current, &name)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("patch change request: read standard change template: %w", err)
	}
	return templateUsableRefusal(stringOrEmpty(state), stringOrEmpty(current), *snap.templateVersionID, stringOrEmpty(name), "approval cannot be requested")
}

// checkStandardChangeCustomerAtApproval refuses (400,
// CodeStandardChangeTemplateOtherCustomer) Request Approval of a templated change
// request whose project (effectiveProject: the request's, else the stored one) is
// not, at this moment, a project of the template's customer. Creation and every
// PATCH keep the project with the template's customer, but the Salesforce project
// sync is the source of truth for a project's account and is not blocked: it may
// re-parent the project after the change was raised. The answer is the same 400
// (and code) as every other "this template is not this project's customer's"
// refusal, so a client tells them apart by the one code.
func checkStandardChangeCustomerAtApproval(ctx context.Context, tx pgx.Tx, snap changeRequestGateSnapshot, effectiveProject *string) error {
	if snap.templateVersionID == nil || (snap.state != "" && snap.state != "NEW") {
		return nil
	}
	var name, accountID, accountName *string
	err := tx.QueryRow(ctx, `
		SELECT t.name, a.id::text, a.name
		  FROM standard_change_template_version v JOIN standard_change_template t ON t.id = v.template_id
		  LEFT JOIN account a ON a.id = t.account_id
		 WHERE v.id = $1::uuid`, *snap.templateVersionID).Scan(&name, &accountID, &accountName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // checkStandardChangeTemplateUsable has answered it
	}
	if err != nil {
		return fmt.Errorf("patch change request: read standard change template: %w", err)
	}
	if err := checkProjectOfTemplateCustomer(ctx, tx, stringOrEmpty(name), accountRef(accountID, accountName), effectiveProject); err != nil {
		var ve *apierror.ValidationError
		if errors.As(err, &ve) && ve.Code == apierror.CodeStandardChangeTemplateOtherCustomer && hasProjectID(effectiveProject) {
			return &apierror.ValidationError{Code: ve.Code, Msg: "approval cannot be requested: " + ve.Msg +
				". The change request's project now belongs to another customer: cancel this change request and raise a new one from that customer's template"}
		}
		return err
	}
	return nil
}

// templateUsableRefusal is the 409 for a template that cannot be used (state is
// the stored upper-case state, "" when the template could not be read), nil when
// it can.
func templateUsableRefusal(state, currentVersionID, versionID, name, what string) error {
	switch {
	case state == "":
		return &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable, Msg: what + ": the standard change template could not be found"}
	case state == "SUSPENDED":
		return &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("%s: the standard change template %q is suspended", what, name)}
	case state != "ACTIVE":
		return &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("%s: the standard change template %q has been retired", what, name)}
	case !strings.EqualFold(currentVersionID, versionID):
		return &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("%s: the standard change template %q has been revised since; cancel this change request and raise a new one from the current version", what, name)}
	}
	return nil
}

// recordChangeRequestTemplate writes the template version and the parameter
// values on a change request being created, in its transaction, after checking
// (under FOR SHARE) that the version is still its template's current one, the
// template is active, and projectID (the change request's project) is one of
// the template's customer's.
func recordChangeRequestTemplate(ctx context.Context, tx pgx.Tx, changeRequestID, versionID string, projectID *string, values map[string]string) error {
	var state, current, name, accountID, accountName *string
	err := tx.QueryRow(ctx, `
		SELECT t.state, t.current_version_id::text, t.name, a.id::text, a.name
		  FROM standard_change_template_version v JOIN standard_change_template t ON t.id = v.template_id
		  LEFT JOIN account a ON a.id = t.account_id
		 WHERE v.id = $1::uuid
		 FOR SHARE OF t`, versionID).Scan(&state, &current, &name, &accountID, &accountName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("create change request: read standard change template: %w", err)
	}
	if err := templateUsableRefusal(stringOrEmpty(state), stringOrEmpty(current), versionID, stringOrEmpty(name), "the change request cannot be raised"); err != nil {
		return err
	}
	if err := checkProjectOfTemplateCustomer(ctx, tx, stringOrEmpty(name), accountRef(accountID, accountName), projectID); err != nil {
		return err
	}
	if values == nil {
		values = map[string]string{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("create change request: encode standard change parameters: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE change_request SET standard_change_template_version_id = $2::uuid, standard_change_parameter_values = $3::jsonb
		 WHERE id = $1::uuid`, changeRequestID, versionID, string(raw)); err != nil {
		return fmt.Errorf("create change request: record standard change template: %w", err)
	}
	return nil
}

// TemplateCustomerRefusal is the 400 (CodeStandardChangeTemplateOtherCustomer)
// for using the standard change template named templateName, whose customer is
// templateCustomer (nil: none recorded), for a project whose customer is
// projectCustomer (nil: no project; an empty ID: a project with no account). nil
// when they are the same customer. Shared with the service, which checks it
// before the previous system is called.
func TemplateCustomerRefusal(templateName string, templateCustomer, projectCustomer *domain.EntityRef) error {
	switch {
	case templateCustomer == nil:
		return &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("the standard change template %q has no customer recorded, so no change request can be raised from it until its customer is set", templateName)}
	case projectCustomer == nil:
		return &apierror.ValidationError{Code: apierror.CodeStandardChangeTemplateOtherCustomer,
			Msg: fmt.Sprintf("projectId is required: the standard change template %q is %s's, and a change request is raised from it only for one of %s's projects", templateName, customerName(templateCustomer), customerName(templateCustomer))}
	case !sameAccount(templateCustomer.ID, projectCustomer.ID):
		return &apierror.ValidationError{Code: apierror.CodeStandardChangeTemplateOtherCustomer,
			Msg: fmt.Sprintf("the standard change template %q is %s's and cannot be used for a project of %s: a template is pre-authorised for its own customer's deployments only",
				templateName, customerName(templateCustomer), customerName(projectCustomer))}
	}
	return nil
}

// checkProjectOfTemplateCustomer reads the project's customer and refuses it when
// it is not the template's (TemplateCustomerRefusal). An unknown project is a 400.
func checkProjectOfTemplateCustomer(ctx context.Context, q crQuerier, templateName string, templateCustomer *domain.EntityRef, projectID *string) error {
	var projectCustomer *domain.EntityRef
	if hasProjectID(projectID) {
		ref, err := projectAccount(ctx, q, *projectID)
		if err != nil {
			return err
		}
		if ref == nil {
			return &apierror.ValidationError{Msg: "projectId does not refer to a project"}
		}
		projectCustomer = ref
	}
	return TemplateCustomerRefusal(templateName, templateCustomer, projectCustomer)
}

// checkStandardChangeProjectCustomer refuses (400,
// CodeStandardChangeTemplateOtherCustomer) a PATCH that moves a change request
// raised from a standard change template to a project of another customer, in
// every state (after New the project is frozen anyway; in New this is what keeps
// the change request with its template's customer). A move to another project of
// the same customer is left to the other rules; resending the stored project is
// accepted.
func checkStandardChangeProjectCustomer(ctx context.Context, tx pgx.Tx, snap changeRequestGateSnapshot, requested *string) error {
	if snap.templateVersionID == nil || requested == nil {
		return nil
	}
	if snap.projectID != nil && strings.EqualFold(strings.TrimSpace(*snap.projectID), strings.TrimSpace(*requested)) {
		return nil
	}
	var name, accountID, accountName *string
	err := tx.QueryRow(ctx, `
		SELECT t.name, a.id::text, a.name
		  FROM standard_change_template_version v JOIN standard_change_template t ON t.id = v.template_id
		  LEFT JOIN account a ON a.id = t.account_id
		 WHERE v.id = $1::uuid`, *snap.templateVersionID).Scan(&name, &accountID, &accountName)
	if err != nil {
		return fmt.Errorf("patch change request: read standard change template: %w", err)
	}
	return checkProjectOfTemplateCustomer(ctx, tx, stringOrEmpty(name), accountRef(accountID, accountName), requested)
}

// flagStandardTemplateOnFailure flags the template a change request was raised
// from for review when THIS write failed the change: it rolled the change back
// (rolledBack: the write moved the state, which is then checked to be ROLLBACK),
// or it recorded a deviation from the implementation plan (deviated: the write
// changed the stored one, which is then checked to be non-blank). A deviation
// stored earlier, or resent unchanged, does not flag again, so a flag the owner
// cleared stays cleared. A no-op for a change request not raised from a template. It escalates
// the transaction's identity (flagStandardTemplateReview), so it runs last in its
// transaction.
func flagStandardTemplateOnFailure(ctx context.Context, tx pgx.Tx, id, actor string, rolledBack, deviated bool) error {
	if !rolledBack && !deviated {
		return nil
	}
	var state, deviation, version *string
	err := tx.QueryRow(ctx, `SELECT state::text, implementation_deviation, standard_change_template_version_id::text
		FROM change_request WHERE id = $1::uuid`, id).Scan(&state, &deviation, &version)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && version == nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("flag standard change template: %w", err)
	}
	switch {
	case rolledBack && strings.EqualFold(stringOrEmpty(state), crStateRollback):
		return flagStandardTemplateReview(ctx, tx, id, "A change request raised from the template was rolled back", actor)
	case deviated && deviation != nil && !IsBlankRichText(*deviation):
		return flagStandardTemplateReview(ctx, tx, id, "A change request raised from the template deviated from its implementation plan", actor)
	}
	return nil
}

// readChangeRequestStandardChange is the detail's standardChange: nil for a change
// request not raised from a template.
func readChangeRequestStandardChange(ctx context.Context, q crQuerier, id string) (*domain.ChangeRequestStandardChange, error) {
	var out domain.ChangeRequestStandardChange
	var accountID, accountName *string
	var raw []byte
	var paramsRaw []byte
	err := q.QueryRow(ctx, `
		SELECT t.id::text, t.name, t.number, a.id::text, a.name, v.id::text, v.version, COALESCE(v.parameters, '[]'::jsonb), cr.standard_change_parameter_values,
		       t.account_id IS DISTINCT FROM p.account_id
		  FROM change_request cr
		  JOIN work_item wi ON wi.id = cr.id
		  LEFT JOIN project p ON p.id = wi.project_id
		  JOIN standard_change_template_version v ON v.id = cr.standard_change_template_version_id
		  JOIN standard_change_template t ON t.id = v.template_id
		  LEFT JOIN account a ON a.id = t.account_id
		 WHERE cr.id = $1::uuid`, id).
		Scan(&out.Template.ID, &out.Template.Name, &out.TemplateNumber, &accountID, &accountName, &out.VersionID, &out.Version, &paramsRaw, &raw,
			&out.CustomerMismatch)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read change request standard change: %w", err)
	}
	out.Customer = accountRef(accountID, accountName)
	var params []domain.StandardChangeParameter
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return nil, fmt.Errorf("read change request standard change: parameters: %w", err)
	}
	values := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("read change request standard change: values: %w", err)
		}
	}
	out.Parameters = make([]domain.ChangeRequestStandardChangeParameter, 0, len(params))
	for _, p := range params {
		cp := domain.ChangeRequestStandardChangeParameter{StandardChangeParameter: p}
		if v, ok := values[p.Key]; ok && v != "" {
			v := v
			cp.Value = &v
		}
		out.Parameters = append(out.Parameters, cp)
	}
	return &out, nil
}

// refuseOtherWorkItemType answers 404 for a work item the caller can see that is
// not a change request (a standard change proposal, whose CAB stage lives in the
// same approval tables). A work item the caller cannot see is left to the caller's
// own visibility handling.
func refuseOtherWorkItemType(ctx context.Context, q crQuerier, id string) error {
	var typ string
	err := q.QueryRow(ctx, `SELECT type::text FROM work_item WHERE id = $1::uuid`, id).Scan(&typ)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check work item type: %w", err)
	}
	if typ != "CHANGE_REQUEST" {
		return &apierror.NotFoundError{Msg: "change request not found"}
	}
	return nil
}
