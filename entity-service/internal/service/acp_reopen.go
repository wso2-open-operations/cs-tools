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
	"encoding/json"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// The ACP reopen check: when Salesforce reports a change to a project or an
// invoice, a project the Account Closure Process suspended goes back to Open
// once the reason it was suspended for is gone, as the previous process did.
//
//   - End-date track (end_date_closure_state CLOSED): reopens when the
//     project's end date is in the future again (a renewal moves it).
//   - Invoice track (invoice_due_date_closure_state SUSPENDED): reopens when
//     no invoice ACP counts is past its suspend date any more (paid).
//
// Only a track ACP suspended is touched: its suspensionProcessState section
// must carry event_type "suspend", which ACP writes when it suspends and a
// manual suspension never does. A project whose overall closure state was set
// by hand (it differs from what the three sub-states give) is left alone. A
// reopened track gets event_type "open", so ACP's reminders start over for
// the new term. The Salesforce project's State__c goes back to Open only once
// the overall closure state is OPEN (every track, compliance included), and
// only then is the "Back to Open state" notice queued (the outbox, published
// by ProjectReactivationDrainer).
//
// It runs two ways: after each Salesforce Project__c / Invoice__c event, and
// once a day over every project ACP suspended (CheckSuspendedProjects, called
// by csm-scheduled-tasks), which catches what an event missed and finishes a
// project a person set back to Open by hand.
//
// Everything is read live from Salesforce, as ACP reads it, so the answer is
// the one ACP would give: the rules are its own, copied in
// acp_reopen_rules.go.

// ACPReopenSalesforce is the part of the REST sales-entity client the reopen
// check uses. Satisfied by *salesentity.Client.
type ACPReopenSalesforce interface {
	GetProject(ctx context.Context, id string) (salesentity.Project, error)
	GetCustomer(ctx context.Context, id string) (salesentity.Customer, error)
	GetInvoice(ctx context.Context, id string) (salesentity.Invoice, error)
	GetOpportunity(ctx context.Context, id string) (salesentity.Opportunity, error)
	LinkedOpportunitiesByProject(ctx context.Context, projectID string) ([]salesentity.LinkedOpportunity, error)
	LinkedOpportunitiesByOpportunity(ctx context.Context, opportunityID string) ([]salesentity.LinkedOpportunity, error)
	InvoicesByOpportunity(ctx context.Context, opportunityID string) ([]salesentity.Invoice, error)
	UpdateProjectStatus(ctx context.Context, projectID, status string) error
}

// ACPReopen bundles the dependencies of the reopen check. It is optional: a
// salesforceEventService without it never reopens anything, which is how
// CSM_ACP_REOPEN_ENABLED=false is realised in routes.go.
type ACPReopen struct {
	Projects    repository.ACPReopenRepository
	SalesEntity ACPReopenSalesforce
	// Notify queues the "Back to Open state" notice (project.reactivated)
	// with the reopen; ProjectReactivationDrainer publishes it. false (no
	// event publisher configured): the project is still reopened, no email.
	Notify bool
	// Mirror copies a reopen to the previous system's project
	// (DATA_SOURCE=postgres-servicenow-dual-write), as PATCH /projects/{id}
	// copies ACP's suspension: the same fields, the same dispatcher. nil in
	// every other mode.
	Mirror func(ctx context.Context, projectID string, req domain.ProjectUpdateRequest)
	// InternalDomain is the address domain the notice may go to ("@wso2.com"
	// when empty): it is an internal notice, and the technical owner
	// recorded in Salesforce is sometimes a customer's person.
	InternalDomain string
	// DryRun decides and logs what it would do, and writes nothing
	// (CSM_ACP_REOPEN_DRY_RUN, default true).
	DryRun bool
	// TestProjectSfID, when set, limits the check to that one Salesforce
	// project; every other project is left alone
	// (CSM_ACP_REOPEN_TEST_PROJECT_SF_ID).
	TestProjectSfID string
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

func (a *ACPReopen) enabled() bool {
	return a != nil && a.Projects != nil && a.SalesEntity != nil
}

func (a *ACPReopen) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// WithACPReopen turns the reopen check on for a service built by one of the
// NewSalesforceEventService constructors and returns it.
func WithACPReopen(svc SalesforceEventService, reopen ACPReopen) SalesforceEventService {
	if s, ok := svc.(*salesforceEventService); ok {
		s.acpReopen = &reopen
	}
	return svc
}

// acpReopenTriggers are the event types after which a project may have
// changed in a way that clears a suspension. A DELETED record cannot.
func acpReopenTriggers(eventType string) bool {
	switch eventType {
	case domain.SalesforceEventCreated, domain.SalesforceEventUpdated, domain.SalesforceEventRestored:
		return true
	}
	return false
}

// reopenAfterProjectEvent runs the reopen check for the project a Project__c
// event names. It runs whether or not the Project ingest is on: the check
// reads Salesforce itself and needs nothing the ingest writes. A failure is
// logged and never fails the event: the ingest already succeeded, and the
// daily check (CheckSuspendedProjects) tries the project again.
func (s *salesforceEventService) reopenAfterProjectEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.acpReopen.enabled() || !acpReopenTriggers(req.EventType) {
		return nil
	}
	if _, err := s.acpReopen.reopenProject(ctx, req.ReferenceID); err != nil {
		slog.ErrorContext(ctx, "acp reopen: check failed, left to the daily check",
			"projectSfId", req.ReferenceID, "err", err)
	}
	return nil
}

// reopenAfterInvoiceEvent runs the reopen check for every project linked to
// the opportunity of the invoice an Invoice__c event names. A failure is
// logged and left to the daily check, as for a project event.
func (s *salesforceEventService) reopenAfterInvoiceEvent(ctx context.Context, req domain.SalesforceEventRequest) error {
	if !s.acpReopen.enabled() || !acpReopenTriggers(req.EventType) {
		return nil
	}
	if err := s.acpReopen.reopenForInvoice(ctx, req.ReferenceID); err != nil {
		slog.ErrorContext(ctx, "acp reopen: check failed, left to the daily check",
			"invoiceSfId", req.ReferenceID, "err", err)
	}
	return nil
}

// reopenForInvoice finds the projects an invoice belongs to (invoice ->
// opportunity -> linked projects) and checks each. It first asks the
// database whether any project has its invoice track suspended by ACP: an
// invoice can only reopen such a project, so most invoice events end there,
// with no Salesforce call. Every project is tried; the first error is
// returned.
func (a *ACPReopen) reopenForInvoice(ctx context.Context, invoiceSfID string) error {
	suspended, err := a.Projects.AnyACPSuspendedInvoiceTrack(ctx)
	if err != nil || !suspended {
		return err
	}
	inv, err := a.SalesEntity.GetInvoice(ctx, invoiceSfID)
	if err != nil {
		return err
	}
	oppID := strings.TrimSpace(derefString(inv.OpportunityID))
	if oppID == "" {
		return nil
	}
	links, err := a.SalesEntity.LinkedOpportunitiesByOpportunity(ctx, oppID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var firstErr error
	for _, l := range links {
		projectSfID := salesforceID18(derefString(l.ProjectID))
		if projectSfID == "" || seen[projectSfID] {
			continue
		}
		seen[projectSfID] = true
		if _, err := a.reopenProject(ctx, projectSfID); err != nil {
			slog.ErrorContext(ctx, "acp reopen: check failed", "projectSfId", projectSfID, "invoiceSfId", invoiceSfID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// acpTracks is which of a project's tracks ACP suspended.
type acpTracks struct {
	endDate bool
	invoice bool
}

// acpSuspendedTracks reads which tracks of p ACP suspended: the track's
// closure state is suspended and its suspensionProcessState section records
// ACP's suspension.
func acpSuspendedTracks(p domain.ACPReopenProject) (acpTracks, error) {
	endEvent, err := acpSectionEventType(p.SuspensionProcessState, acpEndDateSection)
	if err != nil {
		return acpTracks{}, err
	}
	invoiceEvent, err := acpSectionEventType(p.SuspensionProcessState, acpInvoiceSection)
	if err != nil {
		return acpTracks{}, err
	}
	end := p.EndDateClosureState != nil &&
		(*p.EndDateClosureState == "CLOSED" || *p.EndDateClosureState == "SUSPENDED")
	inv := p.InvoiceDueDateClosureState != nil && strings.HasPrefix(*p.InvoiceDueDateClosureState, "SUSPENDED")
	return acpTracks{
		endDate: end && endEvent == acpSuspendEventType,
		invoice: inv && invoiceEvent == acpSuspendEventType,
	}, nil
}

// acpOutcome is what one project's check did.
type acpOutcome int

const (
	acpNothing  acpOutcome = iota // nothing to do (or a dry run)
	acpReopened                   // one or both tracks set back to Open
	acpFinished                   // opened by a person, now finished
)

// acpReopenSalesforceWriteTimeout bounds the Salesforce State__c write made
// inside the reopen transaction, token refresh and retry included, so the
// project row is not held locked for long.
const acpReopenSalesforceWriteTimeout = 20 * time.Second

// acpDerivedClosureState is the overall closure state the three sub-states
// give, the rule UpdateProject and Reopen apply (wso2ClosureStateRule).
func acpDerivedClosureState(p domain.ACPReopenProject) string {
	end := derefString(p.EndDateClosureState)
	inv := derefString(p.InvoiceDueDateClosureState)
	switch {
	case end == "CLOSED" || end == "SUSPENDED" || strings.HasPrefix(inv, "SUSPENDED") ||
		derefString(p.ComplianceViolationClosureState) == "SUSPENDED":
		return "SUSPENDED"
	case end == "RESTRICTED" || strings.HasPrefix(inv, "RESTRICTED"):
		return "RESTRICTED"
	}
	return "OPEN"
}

// acpSetByHand reports whether the project's overall closure state was set
// directly (the admin "Edit closure state" action): it differs from what its
// sub-states give. Such a project is a person's decision and is left alone.
func acpSetByHand(p domain.ACPReopenProject) bool {
	return p.ClosureState != nil && *p.ClosureState != acpDerivedClosureState(p)
}

// reopenProject is the check for one project, by its Salesforce id. It
// returns acpNothing when there is nothing to do, and an error only when a
// retry can help (Salesforce or the database failed); bad data is logged and
// acknowledged.
func (a *ACPReopen) reopenProject(ctx context.Context, rawSfID string) (acpOutcome, error) {
	sfID := salesforceID18(rawSfID)
	if sfID == "" {
		return acpNothing, nil
	}
	if a.TestProjectSfID != "" && !strings.EqualFold(salesforceID18(a.TestProjectSfID), sfID) {
		return acpNothing, nil
	}
	proj, err := a.Projects.GetProjectBySfID(ctx, sfID)
	if err != nil {
		return acpNothing, err
	}
	if proj == nil {
		return acpNothing, nil
	}
	return a.checkProject(ctx, *proj, sfID)
}

// checkProject decides one project read from the database: reopen the tracks
// ACP suspended whose reason is gone, or finish a project a person set back
// to Open while ACP's record still says suspended.
func (a *ACPReopen) checkProject(ctx context.Context, proj domain.ACPReopenProject, sfID string) (acpOutcome, error) {
	suspended, err := acpSuspendedTracks(proj)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: suspensionProcessState is not readable, leaving the project as it is",
			"projectId", proj.ID, "projectSfId", sfID, "err", err)
		return acpNothing, nil
	}
	if acpSetByHand(proj) {
		if suspended.endDate || suspended.invoice {
			slog.InfoContext(ctx, "acp reopen: closure state was set by hand, leaving the project as it is",
				"projectId", proj.ID, "projectSfId", sfID, "closureState", derefString(proj.ClosureState))
		}
		return acpNothing, nil
	}
	if !suspended.endDate && !suspended.invoice {
		if acpOpenedByHand(proj) {
			return a.finishOpened(ctx, proj, sfID)
		}
		return acpNothing, nil
	}

	sp, err := a.SalesEntity.GetProject(ctx, sfID)
	if err != nil {
		return acpNothing, err
	}
	now := a.now()
	var customer *salesentity.Customer
	loadCustomer := func() (*salesentity.Customer, error) {
		if customer != nil {
			return customer, nil
		}
		customerID := strings.TrimSpace(derefString(sp.CustomerID))
		if customerID == "" {
			customer = &salesentity.Customer{}
			return customer, nil
		}
		c, err := a.SalesEntity.GetCustomer(ctx, customerID)
		if err != nil {
			return nil, err
		}
		customer = &c
		return customer, nil
	}

	var reopen acpTracks
	if suspended.endDate {
		reopen.endDate = a.endDateClear(ctx, proj.ID, sp, now)
	}
	if suspended.invoice {
		c, err := loadCustomer()
		if err != nil {
			return acpNothing, err
		}
		clear, err := a.invoiceTrackClear(ctx, proj.ID, sp, c, now)
		if err != nil {
			return acpNothing, err
		}
		reopen.invoice = clear
	}
	if !reopen.endDate && !reopen.invoice {
		slog.InfoContext(ctx, "acp reopen: still suspended, nothing to reopen",
			"projectId", proj.ID, "projectSfId", sfID,
			"endDateSuspended", suspended.endDate, "invoiceSuspended", suspended.invoice)
		return acpNothing, nil
	}
	if a.DryRun {
		slog.InfoContext(ctx, "acp reopen: dry run, would reopen",
			"projectId", proj.ID, "projectSfId", sfID,
			"endDate", reopen.endDate, "invoice", reopen.invoice)
		return acpNothing, nil
	}
	// The notice needs the account's people; read them before anything is
	// written, so a failure here is retried with nothing changed.
	c, err := loadCustomer()
	if err != nil {
		return acpNothing, err
	}

	var written domain.ACPReopenWrite
	plan := func(current domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
		// Re-read under the row lock: ACP or a person may have changed the
		// project since it was read above.
		if acpSetByHand(current) {
			return domain.ACPReopenWrite{}, false, nil
		}
		still, err := acpSuspendedTracks(current)
		if err != nil {
			return domain.ACPReopenWrite{}, false, err
		}
		w := domain.ACPReopenWrite{
			ReopenEndDate:          reopen.endDate && still.endDate,
			ReopenInvoice:          reopen.invoice && still.invoice,
			SuspensionProcessState: current.SuspensionProcessState,
		}
		if !w.ReopenEndDate && !w.ReopenInvoice {
			return w, false, nil
		}
		if w.ReopenEndDate {
			if w.SuspensionProcessState, err = acpWithSectionOpen(w.SuspensionProcessState, acpEndDateSection); err != nil {
				return domain.ACPReopenWrite{}, false, err
			}
		}
		if w.ReopenInvoice {
			if w.SuspensionProcessState, err = acpWithSectionOpen(w.SuspensionProcessState, acpInvoiceSection); err != nil {
				return domain.ACPReopenWrite{}, false, err
			}
		}
		written = w
		return w, true, nil
	}
	outcome, done, err := a.Projects.Reopen(ctx, proj.ID, plan, a.salesforceOpenBeforeCommit(sp.ID),
		func(outcome domain.ACPReopenOutcome) (json.RawMessage, error) {
			if !outcome.Open {
				return nil, nil
			}
			return a.notice(ctx, proj, sp, c, written, now)
		})
	if err != nil {
		return acpNothing, err
	}
	if !done {
		return acpNothing, nil
	}
	slog.InfoContext(ctx, "acp reopen: reopened",
		"projectId", proj.ID, "projectSfId", sfID,
		"endDate", written.ReopenEndDate, "invoice", written.ReopenInvoice, "projectOpen", outcome.Open)
	a.mirror(ctx, proj.ID, written)
	return acpReopened, nil
}

// acpOpenedByHand reports whether a project is Open with every track open,
// while a section still records ACP's suspension: a person set the
// suspended track back to Open (PATCH /projects/{id}), which neither sets
// the Salesforce project to Open nor sends the notice. The daily check
// finishes it.
func acpOpenedByHand(p domain.ACPReopenProject) bool {
	if derefString(p.ClosureState) != "OPEN" || acpDerivedClosureState(p) != "OPEN" {
		return false
	}
	for _, key := range []string{acpEndDateSection, acpInvoiceSection} {
		if ev, err := acpSectionEventType(p.SuspensionProcessState, key); err == nil && ev == acpSuspendEventType {
			return true
		}
	}
	return false
}

// finishOpened finishes a project a person set back to Open: ACP's record of
// the suspension is set to "open", the Salesforce project to Open, and the
// notice is queued (with no reason: the project was not reopened by a
// renewal or a payment seen here).
func (a *ACPReopen) finishOpened(ctx context.Context, proj domain.ACPReopenProject, sfID string) (acpOutcome, error) {
	if a.DryRun {
		slog.InfoContext(ctx, "acp reopen: dry run, would finish a project set back to Open by hand",
			"projectId", proj.ID, "projectSfId", sfID)
		return acpNothing, nil
	}
	sp, err := a.SalesEntity.GetProject(ctx, sfID)
	if err != nil {
		return acpNothing, err
	}
	var c *salesentity.Customer
	if customerID := strings.TrimSpace(derefString(sp.CustomerID)); customerID != "" {
		cust, err := a.SalesEntity.GetCustomer(ctx, customerID)
		if err != nil {
			return acpNothing, err
		}
		c = &cust
	}
	now := a.now()
	var written domain.ACPReopenWrite
	plan := func(current domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
		if !acpOpenedByHand(current) {
			return domain.ACPReopenWrite{}, false, nil
		}
		state := current.SuspensionProcessState
		var err error
		for _, key := range []string{acpEndDateSection, acpInvoiceSection} {
			if ev, _ := acpSectionEventType(state, key); ev == acpSuspendEventType {
				if state, err = acpWithSectionOpen(state, key); err != nil {
					return domain.ACPReopenWrite{}, false, err
				}
			}
		}
		written = domain.ACPReopenWrite{SectionsOnly: true, SuspensionProcessState: state}
		return written, true, nil
	}
	outcome, done, err := a.Projects.Reopen(ctx, proj.ID, plan, a.salesforceOpenBeforeCommit(sp.ID),
		func(outcome domain.ACPReopenOutcome) (json.RawMessage, error) {
			if !outcome.Open {
				return nil, nil
			}
			return a.notice(ctx, proj, sp, c, written, now)
		})
	if err != nil {
		return acpNothing, err
	}
	if !done {
		return acpNothing, nil
	}
	slog.InfoContext(ctx, "acp reopen: finished a project set back to Open by hand",
		"projectId", proj.ID, "projectSfId", sfID, "projectOpen", outcome.Open)
	a.mirror(ctx, proj.ID, written)
	return acpFinished, nil
}

// salesforceOpenBeforeCommit sets the Salesforce project to Open inside the
// reopen transaction, once the project is Open overall: if Salesforce
// refuses, the database is rolled back and the daily check tries again. The
// call has its own deadline (acpReopenSalesforceWriteTimeout), so the row
// lock is held at most that long. If the commit then fails after Salesforce
// accepted, the daily check finds the project still suspended in CSM and
// sets Salesforce to Open again, which is harmless.
func (a *ACPReopen) salesforceOpenBeforeCommit(salesforceProjectID string) func(context.Context, domain.ACPReopenOutcome) error {
	return func(ctx context.Context, outcome domain.ACPReopenOutcome) error {
		if !outcome.Open {
			return nil
		}
		writeCtx, cancel := context.WithTimeout(ctx, acpReopenSalesforceWriteTimeout)
		defer cancel()
		return a.SalesEntity.UpdateProjectStatus(writeCtx, salesforceProjectID, salesentity.ProjectStatusOpen)
	}
}

// mirror copies a committed reopen to the previous system's project
// (dual-write only): the tracks set back to Open and the new
// suspensionProcessState, as PATCH /projects/{id} copies ACP's suspension.
func (a *ACPReopen) mirror(ctx context.Context, projectID string, w domain.ACPReopenWrite) {
	if a.Mirror == nil {
		return
	}
	open := "Open"
	req := domain.ProjectUpdateRequest{SuspensionProcessState: w.SuspensionProcessState}
	if w.ReopenEndDate {
		req.EndDateClosureState = &open
	}
	if w.ReopenInvoice {
		req.InvoiceDueDateClosureState = &open
	}
	a.Mirror(ctx, projectID, req)
}

// notice builds the queued "Back to Open state" notice, nil when notices are
// off.
func (a *ACPReopen) notice(ctx context.Context, p domain.ACPReopenProject, sp salesentity.Project, c *salesentity.Customer, w domain.ACPReopenWrite, now time.Time) (json.RawMessage, error) {
	if !a.Notify {
		slog.InfoContext(ctx, "acp reopen: no event publisher, no reactivation email", "projectId", p.ID)
		return nil, nil
	}
	payload := buildProjectReactivatedPayload(p, sp, c, w, now, a.internalDomain())
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *ACPReopen) internalDomain() string {
	if d := strings.TrimSpace(a.InternalDomain); d != "" {
		return strings.ToLower(d)
	}
	return wso2EmailDomain
}

// acpCheckDefaultLimit and acpCheckMaxLimit bound one page of the daily
// check; acpCheckTimeReserve is how much of the request's deadline is kept
// back, so a page stops before a project's check would be cut off. The first
// project of a page is always checked, whatever the time left, so every call
// makes progress even under a short REQUEST_TIMEOUT.
const (
	acpCheckDefaultLimit = 10
	acpCheckMaxLimit     = 50
	acpCheckTimeReserve  = 25 * time.Second
)

// CheckSuspendedProjects is the daily safety net (POST /acp/reopen-check):
// every project ACP's record still shows suspended is checked as an event
// would check it. It catches a reopen an event missed (lost, or sent while
// the check was off or in a dry run), a reopen that failed, a project ACP
// suspended on stale data, and a project a person set back to Open, which it
// finishes. One page per call; the caller calls again from NextCursor while
// More is true.
func (a *ACPReopen) CheckSuspendedProjects(ctx context.Context, req domain.ACPReopenCheckRequest) (domain.ACPReopenCheckResponse, error) {
	resp := domain.ACPReopenCheckResponse{FailedProjectIDs: []string{}}
	limit := req.Limit
	if limit <= 0 {
		limit = acpCheckDefaultLimit
	}
	if limit > acpCheckMaxLimit {
		limit = acpCheckMaxLimit
	}
	if req.Cursor != "" {
		if err := validateUUIDs("cursor", []string{req.Cursor}); err != nil {
			return resp, err
		}
	}
	projects, err := a.Projects.ListACPNotedProjects(ctx, req.Cursor, limit)
	if err != nil {
		return resp, err
	}
	for i, p := range projects {
		if deadline, ok := ctx.Deadline(); ok && i > 0 && time.Until(deadline) < acpCheckTimeReserve {
			// Out of time: the next call starts at the first project not done.
			resp.NextCursor, resp.More = projects[i-1].ID, true
			return resp, nil
		}
		resp.Checked++
		sfID := salesforceID18(p.SfID)
		if a.TestProjectSfID != "" && !strings.EqualFold(salesforceID18(a.TestProjectSfID), sfID) {
			continue
		}
		outcome, err := a.checkProject(ctx, p, sfID)
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "acp reopen: daily check failed for a project", "projectId", p.ID, "projectSfId", sfID, "err", err)
			resp.Failed++
			resp.FailedProjectIDs = append(resp.FailedProjectIDs, p.ID)
		case outcome == acpReopened:
			resp.Reopened++
		case outcome == acpFinished:
			resp.Finished++
		}
	}
	if len(projects) == limit {
		resp.NextCursor, resp.More = projects[len(projects)-1].ID, true
	}
	return resp, nil
}

// endDateClear reports whether the end-date track may reopen: the project's
// end date is set and in the future.
func (a *ACPReopen) endDateClear(ctx context.Context, projectID string, sp salesentity.Project, now time.Time) bool {
	end, err := acpParseDate(sp.EndDate)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: project end date is not a date, end-date track stays suspended",
			"projectId", projectID, "projectSfId", sp.ID, "err", err)
		return false
	}
	return end != nil && acpDaysBetween(now, *end) > 0
}

// invoiceTrackClear reads the project's invoices from Salesforce, as ACP
// reads them, and reports whether the invoice track may reopen. An invoice
// date that cannot be read keeps the track suspended (never reopen on data
// that cannot be checked).
func (a *ACPReopen) invoiceTrackClear(ctx context.Context, projectID string, sp salesentity.Project, c *salesentity.Customer, now time.Time) (bool, error) {
	start, err := acpParseDate(sp.StartDate)
	if err != nil {
		slog.WarnContext(ctx, "acp reopen: project start date is not a date, invoice track stays suspended",
			"projectId", projectID, "projectSfId", sp.ID, "err", err)
		return false, nil
	}
	isPartner := strings.EqualFold(strings.TrimSpace(derefString(c.AccountClassification)), "Partner")
	hasPrimaryPartner := strings.TrimSpace(derefString(c.PrimaryPartnerAccountID)) != ""

	links, err := a.SalesEntity.LinkedOpportunitiesByProject(ctx, sp.ID)
	if err != nil {
		return false, err
	}
	var invoices []acpInvoice
	seen := map[string]bool{}
	for _, l := range links {
		oppID := strings.TrimSpace(derefString(l.OpportunityID))
		if oppID == "" || seen[oppID] {
			continue
		}
		seen[oppID] = true
		opp, err := a.SalesEntity.GetOpportunity(ctx, oppID)
		if err != nil {
			return false, err
		}
		if !acpOpportunityCounts(opp.StageName, opp.EulaVersion.Value) {
			continue
		}
		eula := acpEULAVersionDecimal(opp.EulaVersion.Value)
		rows, err := a.SalesEntity.InvoicesByOpportunity(ctx, oppID)
		if err != nil {
			return false, err
		}
		for _, inv := range rows {
			parsed, ok := acpParseInvoice(inv, eula)
			if !ok {
				slog.WarnContext(ctx, "acp reopen: an invoice date is not a date, invoice track stays suspended",
					"projectId", projectID, "projectSfId", sp.ID, "invoiceSfId", derefString(inv.ID))
				return false, nil
			}
			invoices = append(invoices, parsed)
		}
	}
	return acpInvoiceTrackClear(now, invoices, start, isPartner, hasPrimaryPartner), nil
}

// acpParseInvoice reads the dates of one Salesforce invoice; ok is false when
// one of them is not a date.
func acpParseInvoice(inv salesentity.Invoice, eula float64) (acpInvoice, bool) {
	out := acpInvoice{Name: inv.Name, Classification: inv.Classification, EULAVersion: eula}
	var err error
	if out.InvoiceDate, err = acpParseDate(inv.InvoiceDate); err != nil {
		return acpInvoice{}, false
	}
	if out.DueDate, err = acpParseDate(inv.DueDate); err != nil {
		return acpInvoice{}, false
	}
	if out.PaidDate, err = acpParseDate(inv.PaidDate); err != nil {
		return acpInvoice{}, false
	}
	return out, true
}

// buildProjectReactivatedPayload assembles the notice from the Salesforce
// project and account as they were read before the reopen; the name falls
// back to CSM's. The name is kept as it is, "[S]" included, as the legacy
// email showed it: Salesforce removes the marker after the reopen. Only
// well-formed addresses in internalDomain are kept as recipients: it is an
// internal notice, and the notification service rejects the whole event for
// one bad address.
func buildProjectReactivatedPayload(p domain.ACPReopenProject, sp salesentity.Project, c *salesentity.Customer, w domain.ACPReopenWrite, now time.Time, internalDomain string) events.ProjectReactivatedPayload {
	name := strings.TrimSpace(derefString(sp.Name))
	if name == "" {
		name = p.Name
	}
	key := strings.TrimSpace(derefString(sp.Key))
	if key == "" {
		key = p.Key
	}
	payload := events.ProjectReactivatedPayload{
		ProjectID:     p.ID,
		ProjectSfID:   sp.ID,
		ProjectName:   name,
		ProjectKey:    key,
		StartDate:     acpDateOnly(sp.StartDate),
		EndDate:       acpDateOnly(sp.EndDate),
		Reasons:       []string{},
		Recipients:    []string{},
		ReactivatedOn: now.UTC().Format(time.RFC3339),
	}
	if payload.ProjectSfID == "" {
		payload.ProjectSfID = p.SfID
	}
	if w.ReopenEndDate {
		payload.Reasons = append(payload.Reasons, events.ProjectReactivatedReasonRenewal)
	}
	if w.ReopenInvoice {
		payload.Reasons = append(payload.Reasons, events.ProjectReactivatedReasonInvoicePaid)
	}
	if c == nil {
		return payload
	}
	if c.Owner != nil {
		payload.AccountOwnerName = strings.TrimSpace(derefString(c.Owner.Name))
		if e, ok := acpInternalAddress(derefString(c.Owner.Email), internalDomain); ok {
			payload.AccountOwnerEmail = e
		}
	}
	seen := map[string]bool{}
	add := func(email *string) {
		e, ok := acpInternalAddress(derefString(email), internalDomain)
		if !ok || seen[strings.ToLower(e)] {
			return
		}
		seen[strings.ToLower(e)] = true
		payload.Recipients = append(payload.Recipients, e)
	}
	if c.Owner != nil {
		add(c.Owner.Email)
	}
	if c.RenewalManager != nil {
		add(c.RenewalManager.Email)
	}
	add(c.TechnicalOwner)
	return payload
}

// acpDateOnly is the date part of a Salesforce date or datetime, "" when blank.
func acpDateOnly(v *string) string {
	s := strings.TrimSpace(derefString(v))
	if len(s) > 10 {
		s = s[:10]
	}
	return s
}

// acpInternalAddress returns the address when it is a plain, well-formed
// address in domain (case-insensitive), and false otherwise.
func acpInternalAddress(raw, domain string) (string, bool) {
	e := strings.TrimSpace(raw)
	if e == "" {
		return "", false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Name != "" || addr.Address != e {
		return "", false
	}
	if domain == "" || !strings.HasSuffix(strings.ToLower(e), strings.ToLower(domain)) {
		return "", false
	}
	return e, true
}
