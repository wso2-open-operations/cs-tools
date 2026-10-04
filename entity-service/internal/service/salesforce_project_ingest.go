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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// SalesEntityProjectClient fetches one Salesforce Project__c from REST
// sales/sales-entity-service.
type SalesEntityProjectClient interface {
	GetProject(ctx context.Context, id string) (salesentity.Project, error)
}

// ProjectLookup resolves a CSM project id by Salesforce Project__c Id. It is
// a read-only slice of repository.SalesforceProjectRepository, kept separate
// so EnsureProject can read projects even when the Project ingest is off.
type ProjectLookup interface {
	LookupProjectIDBySfID(ctx context.Context, sfID string) (*string, error)
}

// ProjectIngest bundles the dependencies of the Project__c branch of
// POST /salesforce/events. It is optional: a service without it acknowledges
// Project__c envelopes and does nothing, which is how
// CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED=false is realised in
// routes.go.
type ProjectIngest struct {
	Projects    repository.SalesforceProjectRepository
	SalesEntity SalesEntityProjectClient
	// InsertEnabled lets the ingest create projects CSM does not have
	// (CSM_MIGRATION_SALESFORCE_PROJECT_INSERT_ENABLED). Off, the ingest is
	// update-only: csm-sync-service still inserts project rows from
	// ServiceNow, with ids derived from the sys_id, and project.key is
	// UNIQUE, so an ingest-created row would make its insert fail forever
	// (SALESFORCE_SYNC_PLAN.md §6). Turned on at cutover, when csm-sync-service
	// stops.
	InsertEnabled bool
}

func (p *ProjectIngest) enabled() bool {
	return p != nil && p.Projects != nil && p.SalesEntity != nil
}

// errProjectIngestDisabled is what a project re-run returns when the service
// was built without ProjectIngest; the retry job only registers the retrier
// when the flag is on, so seeing it means a wiring mistake.
var errProjectIngestDisabled = errors.New("salesforce: project ingest is disabled")

// errProjectKeyMissing marks a project refused under decision D5: it has no
// Project_Key__c, and project.key is NOT NULL UNIQUE. No redelivery can fix
// the record, so the event is recorded FAILED and acknowledged.
var errProjectKeyMissing = errors.New("salesforce project has no Project_Key__c; project.key is NOT NULL UNIQUE, so it is refused (decision D5)")

// ProjectReingester re-runs the Project ingest for one Salesforce Project__c
// id as if an UPDATED event had arrived. The delayed-retry job registers it
// under domain.SalesforceIngestEntityProject.
type ProjectReingester interface {
	RetryProjectIngest(ctx context.Context, projectSfID string) error
}

// WithProjectIngest turns on the Project__c branch of a service built by one
// of the NewSalesforceEventService constructors and returns it.
func WithProjectIngest(svc SalesforceEventService, ingest ProjectIngest) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.project = &ingest
	}
	return svc
}

// RetryProjectIngest implements ProjectReingester.
func (s *salesforceEventService) RetryProjectIngest(ctx context.Context, projectSfID string) error {
	if !s.project.enabled() {
		return errProjectIngestDisabled
	}
	return ackRefusedProject(s.upsertProject(ctx, projectSfID, domain.SalesforceEventUpdated, true))
}

// projectKeyMissingError is EnsureProject's D5 refusal: a ValidationError
// that also matches errProjectKeyMissing, so a child ingest (linked
// opportunity) can acknowledge it through ackRefusedProject.
type projectKeyMissingError struct {
	*apierror.ValidationError
}

// Unwrap exposes both the ValidationError and the errProjectKeyMissing sentinel.
func (e *projectKeyMissingError) Unwrap() []error {
	return []error{e.ValidationError, errProjectKeyMissing}
}

// ackRefusedProject turns a D5 refusal (already recorded FAILED) into an
// acknowledgement: a keyless project is fixed in Salesforce, not by retrying.
func ackRefusedProject(err error) error {
	if errors.Is(err, errProjectKeyMissing) {
		return nil
	}
	return err
}

// projectInsertsAllowed reports whether this service may create project rows:
// the Project ingest is on and so is its insert switch.
func (s *salesforceEventService) projectInsertsAllowed() bool {
	return s.project.enabled() && s.project.InsertEnabled
}

// handleProjectEvent is the Project__c branch of HandleEvent.
func (s *salesforceEventService) handleProjectEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.project.enabled() {
		slog.InfoContext(ctx, "salesforce: project ingest disabled, ignoring project event",
			"eventType", req.EventType, "referenceId", req.ReferenceID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: project event", "eventType", req.EventType, "referenceId", req.ReferenceID)

	switch req.EventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return ackRefusedProject(s.upsertProject(ctx, req.ReferenceID, req.EventType, true))
	case domain.SalesforceEventDeleted:
		return s.softDeleteProject(ctx, req.ReferenceID)
	case domain.SalesforceEventUndefined:
		return &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}
	default:
		return &apierror.ValidationError{Msg: "eventType must be CREATED, UPDATED, DELETED, RESTORED, or UNDEFINED"}
	}
}

// EnsureProject returns the CSM id of the project with this Salesforce
// Project__c Id, for a child ingest (membership, linked opportunity) that
// needs its parent row first. The project is looked up by sf_id; when it is
// absent and project inserts are allowed (the Project ingest and its insert
// switch are both on) the Project upsert runs first — fetch from Sales
// Entity, write by natural key — and the lookup is repeated. Otherwise the
// project can only arrive through the ServiceNow sync, so the result is a
// NotFoundError whose text ("project not found for sfId ...") the
// delayed-retry job matches.
func (s *salesforceEventService) EnsureProject(ctx context.Context, sfID string) (string, error) {
	sfID = salesforceID18(sfID)
	if sfID == "" {
		return "", &apierror.ValidationError{Msg: "project sfId is required"}
	}
	var lookup ProjectLookup = s.support.Projects
	if lookup == nil && s.project.enabled() {
		lookup = s.project.Projects
	}
	if lookup == nil {
		return "", errors.New("salesforce: project lookup is not configured")
	}
	id, err := lookup.LookupProjectIDBySfID(ctx, sfID)
	if err != nil {
		return "", err
	}
	if id != nil {
		return *id, nil
	}
	if !s.projectInsertsAllowed() {
		return "", &apierror.NotFoundError{Msg: fmt.Sprintf("project not found for sfId %q", sfID)}
	}
	slog.InfoContext(ctx, "salesforce: parent project not in CSM yet, ingesting it first", "projectSfId", sfID)
	// The duplicate guard is off: the row is known to be missing, so a
	// ledger row saying this version was written must not stop the write.
	if err := s.upsertProject(ctx, sfID, domain.SalesforceEventUpdated, false); err != nil {
		if errors.Is(err, errProjectKeyMissing) {
			return "", &projectKeyMissingError{ValidationError: &apierror.ValidationError{Msg: fmt.Sprintf("project %s: %s", sfID, err)}}
		}
		return "", err
	}
	id, err = lookup.LookupProjectIDBySfID(ctx, sfID)
	if err != nil {
		return "", err
	}
	if id == nil {
		return "", fmt.Errorf("salesforce: project %s was upserted but cannot be read back by sf_id", sfID)
	}
	return *id, nil
}

// ensureMissingProjectForMembership is the membership ingest's recovery from
// "project not found": when project inserts are allowed it ingests the
// membership's project (EnsureProject) and reports true so the caller retries
// the membership write once. Otherwise, or when the project cannot be
// ensured, it reports false and the caller keeps the original NotFoundError,
// which the delayed-retry job re-runs.
func (s *salesforceEventService) ensureMissingProjectForMembership(ctx context.Context, cause error, membershipSfID, projectSfID string) bool {
	var nf *apierror.NotFoundError
	if !errors.As(cause, &nf) || !strings.HasPrefix(strings.ToLower(nf.Msg), "project not found") {
		return false
	}
	if strings.TrimSpace(projectSfID) == "" || !s.projectInsertsAllowed() {
		return false
	}
	if _, err := s.EnsureProject(ctx, projectSfID); err != nil {
		slog.WarnContext(ctx, "salesforce: membership's project could not be ingested, leaving the membership to the retry job",
			"membershipSfId", membershipSfID, "projectSfId", projectSfID, "err", err)
		return false
	}
	slog.InfoContext(ctx, "salesforce: membership's project ingested, retrying the membership", "membershipSfId", membershipSfID, "projectSfId", projectSfID)
	return true
}

// upsertProject is the CREATED/UPDATED/RESTORED branch of the Project ingest
// (and EnsureProject's write): read the Project__c from Sales Entity, skip
// it when the ledger already holds this version (guard), otherwise write the
// ten Salesforce-owned columns and a SUCCEEDED ledger row in one
// transaction. A failure is recorded FAILED in the ledger and returned so
// Service Bus redelivers; a keyless project is recorded FAILED and returned
// as errProjectKeyMissing, which the callers acknowledge.
func (s *salesforceEventService) upsertProject(ctx context.Context, sfID, eventType string, guard bool) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	p, err := s.project.SalesEntity.GetProject(ctx, sfID)
	if err != nil {
		return err
	}
	// Sales Entity may answer with the 18-character form of a 15-character
	// referenceId; the stored sf_id is whatever Salesforce returns.
	if strings.TrimSpace(p.ID) != "" {
		sfID = strings.TrimSpace(p.ID)
	}

	eventModifiedOn, ok := parseSalesforceLastModified(p.LastModifiedDate)
	if !ok {
		eventModifiedOn = time.Now().UTC()
	}
	if guard {
		var skip bool
		skip, eventModifiedOn, err = shouldSkipIngest(ctx, s.support.States, domain.SalesforceIngestEntityProject, sfID, eventType, p.LastModifiedDate)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityProject,
		SfID:            sfID,
		EventModifiedOn: eventModifiedOn,
		EventType:       eventType,
		Status:          domain.SalesforceIngestSucceeded,
	}

	key := strings.TrimSpace(derefString(p.Key))
	if key == "" {
		slog.WarnContext(ctx, "salesforce: project has no Project_Key__c, refusing it (decision D5)", "projectSfId", sfID, "eventType", eventType)
		s.recordProjectFailed(ctx, state, errProjectKeyMissing)
		return errProjectKeyMissing
	}

	row := domain.SalesforceProjectUpsert{
		SfID:                    sfID,
		Key:                     truncateString(ctx, "project.key", sfID, key, maxProjectKeyChars),
		Name:                    truncateSfColumn(ctx, "project.name", sfID, optionalPtr(p.Name), maxProjectNameChars),
		StartDate:               parseSalesforceDate(ctx, "startDate", sfID, p.StartDate),
		EndDate:                 parseSalesforceDate(ctx, "endDate", sfID, p.EndDate),
		Description:             optionalPtr(p.Description),
		ComplianceViolationDate: parseSalesforceDate(ctx, "complianceViolationDate", sfID, p.ComplianceViolationDate),
		GoLiveDate:              parseSalesforceDate(ctx, "goLiveDate", sfID, p.GoLiveDate),
		Reactivate:              eventType == domain.SalesforceEventRestored,
		AllowInsert:             s.project.InsertEnabled,
	}
	if row.Name == nil {
		slog.WarnContext(ctx, "salesforce: project has no name", "projectSfId", sfID)
	}
	if customerID := strings.TrimSpace(derefString(p.CustomerID)); customerID != "" {
		accountID, err := s.EnsureAccount(ctx, customerID)
		if err != nil {
			// EnsureAccount's NotFoundError carries the "account not found
			// for sfId" prefix the retry job matches.
			s.recordProjectFailed(ctx, state, err)
			return err
		}
		row.AccountID = &accountID
	} else {
		slog.WarnContext(ctx, "salesforce: project has no customerId, keeping the stored account", "projectSfId", sfID)
	}
	if typeName := strings.TrimSpace(derefString(p.Type)); typeName != "" {
		row.ProjectTypeID, err = s.project.Projects.LookupProjectTypeIDByName(ctx, typeName)
		if err != nil {
			s.recordProjectFailed(ctx, state, err)
			return err
		}
		if row.ProjectTypeID == nil {
			// Never created here: project_type rows carry feature
			// entitlements only a migration can set.
			slog.WarnContext(ctx, "salesforce: project type has no project_type row, writing NULL", "projectSfId", sfID, "projectType", typeName)
		}
	}

	res, err := s.project.Projects.UpsertFromSalesforce(ctx, row, state)
	if err != nil {
		s.recordProjectFailed(ctx, state, err)
		return err
	}
	slog.InfoContext(ctx, "salesforce: project ingested",
		"projectSfId", sfID, "key", row.Key, "projectId", res.ProjectID, "created", res.Created,
		"linkedByKey", res.LinkedByKey, "reactivated", res.Reactivated)
	s.requeueChildrenOf(ctx, repository.MissingParent{Kind: repository.MissingParentProject, SfID: sfID, Key: row.Key})
	return nil
}

// The VARCHAR widths of project.key and project.name (migration 0014).
const (
	maxProjectKeyChars  = 100
	maxProjectNameChars = 255
)

// truncateString is truncateSfColumn for a required value.
func truncateString(ctx context.Context, column, sfID, v string, max int) string {
	return *truncateSfColumn(ctx, column, sfID, &v, max)
}

// softDeleteProject is DELETED: the project is marked inactive
// (is_active = FALSE) rather than removed — its cases, contacts and
// deployments still reference it — and the DELETED ledger row is written in
// the same transaction. A deleted record cannot be read from Sales Entity,
// so there is no LastModifiedDate: the ledger version is the current time,
// or the recorded one when later, so the DELETED row always replaces the
// one before it, as softDeleteAccount does. A 15-character referenceId is
// widened first, so the marker lands on the sf_id the ingest stored.
func (s *salesforceEventService) softDeleteProject(ctx context.Context, sfID string) error {
	if s.support.States == nil {
		return errors.New("salesforce: salesforce_ingest_state ledger is not configured")
	}
	sfID = salesforceID18(sfID)
	if s.project.SalesEntity == nil {
		return errDeleteUnconfirmable
	}
	_, fetchErr := s.project.SalesEntity.GetProject(ctx, sfID)
	if gone, err := confirmDeletedUpstream(ctx, string(domain.SalesforceIngestEntityProject), sfID, fetchErr); err != nil || !gone {
		return err
	}
	modifiedOn := time.Now().UTC()
	st, err := s.support.States.Get(ctx, domain.SalesforceIngestEntityProject, sfID)
	if err != nil {
		return err
	}
	if st != nil && st.EventModifiedOn.After(modifiedOn) {
		modifiedOn = st.EventModifiedOn
	}
	state := domain.UpsertSalesforceIngestStateRequest{
		Entity:          domain.SalesforceIngestEntityProject,
		SfID:            sfID,
		EventModifiedOn: modifiedOn,
		EventType:       domain.SalesforceEventDeleted,
		Status:          domain.SalesforceIngestSucceeded,
	}
	found, err := s.project.Projects.SoftDeleteBySfID(ctx, sfID, state)
	if err != nil {
		s.recordProjectFailed(ctx, state, err)
		return err
	}
	if !found {
		slog.InfoContext(ctx, "salesforce: DELETED project is not in CSM, nothing to mark", "projectSfId", sfID)
		return nil
	}
	slog.InfoContext(ctx, "salesforce: project marked inactive", "projectSfId", sfID)
	return nil
}

// recordProjectFailed writes a FAILED ledger row best-effort, outside the
// rolled-back transaction, so the failure is visible and the delayed-retry
// job can re-run a missing-parent failure. The original error is what the
// caller returns regardless.
func (s *salesforceEventService) recordProjectFailed(ctx context.Context, state domain.UpsertSalesforceIngestStateRequest, cause error) {
	if s.support.States == nil {
		return
	}
	msg := truncateOnboardingStepError(cause.Error())
	state.Status = domain.SalesforceIngestFailed
	state.LastError = &msg
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := s.support.States.Upsert(recordCtx, state); err != nil {
		slog.ErrorContext(ctx, "salesforce: recording FAILED project ingest state also failed", "projectSfId", state.SfID, "err", err)
	}
}
