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

package service

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The two ServiceNow flows this file ports. Both are "Incident Updated where
// State changes to X", run for every update, one step each:
//
//   - Create Incident Report Task (-> In Progress): Create Record on
//     incident_task with the fields reportTaskFor sets.
//   - Incident Report Generator (-> Resolved): Update Record on incident,
//     setting u_incident_report to the template renderIncidentReport builds.
//
// ServiceNow keeps running its own copies against ServiceNow; these write the
// Postgres side, which is what the portal reads. Neither ever calls
// ServiceNow.
const (
	incidentStateInProgress = "IN_PROGRESS"
	incidentStateResolved   = "RESOLVED"

	// incidentReportActor is who the writes are attributed to. The
	// ServiceNow flows run as system, so their records show sys_created_by
	// "system" too.
	incidentReportActor = "system"

	incidentReportBatchSize = 100
)

// IncidentReportMaxAttempts is how many failed attempts a change gets before
// the drainer parks it. Attempts back off exponentially (see
// IncidentReportRepository.PendingChanges), so this spans about three hours
// -- long enough to ride out an outage, not so long that a broken row is
// retried forever. A parked row keeps its reason in event_outbox.last_error
// and is re-driven by setting published_on back to NULL and attempts to 0.
const IncidentReportMaxAttempts = 10

// IncidentReportService applies the incident report flows to one incident
// state change.
type IncidentReportService interface {
	// HandleChange runs whichever flow the change triggers, if any. A change
	// that triggers nothing returns nil, so the row is marked processed.
	HandleChange(ctx context.Context, tx repository.IncidentReportTx, c repository.IncidentReportChange) error
}

type incidentReportService struct{}

// NewIncidentReportService constructs the flow logic.
func NewIncidentReportService() IncidentReportService {
	return &incidentReportService{}
}

// HandleChange implements IncidentReportService. Every recorded change is
// acted on however late it is drained -- the drainer always runs, so there
// is no backlog from a switched-off period to guard against.
func (s *incidentReportService) HandleChange(ctx context.Context, tx repository.IncidentReportTx, c repository.IncidentReportChange) error {
	to, ok := stateChangedTo(c.Changes)
	if !ok || (to != incidentStateInProgress && to != incidentStateResolved) {
		return nil
	}
	src, err := tx.IncidentSource(ctx, c.IncidentID)
	if errors.Is(err, repository.ErrIncidentNotFound) {
		// Deleted since the change was recorded: nothing left to act on.
		slog.InfoContext(ctx, "incidentreport: incident gone, skipping",
			"outboxId", c.OutboxID, "incidentId", c.IncidentID)
		return nil
	}
	if err != nil {
		return err
	}

	switch to {
	case incidentStateInProgress:
		id, number, err := tx.CreateReportTask(ctx, reportTaskFor(src))
		if err != nil {
			return err
		}
		slog.InfoContext(ctx, "incidentreport: created report task",
			"incidentId", c.IncidentID, "taskId", id, "taskNumber", number)
	case incidentStateResolved:
		// Overwrites unconditionally, as the ServiceNow flow does.
		if err := tx.SetIncidentReport(ctx, c.IncidentID, renderIncidentReport(src), incidentReportActor); err != nil {
			return err
		}
		slog.InfoContext(ctx, "incidentreport: wrote incident report", "incidentId", c.IncidentID)
		if err := postResolutionTasks(ctx, tx, src); err != nil {
			return err
		}
	}
	return nil
}

// stateChangedTo reports the new state when the change moved incident.state.
// trg_event_outbox only lists columns that differ, so a "state" entry is
// already a real change -- ServiceNow's "changes to".
func stateChangedTo(changes map[string]map[string]any) (string, bool) {
	st, ok := changes["state"]
	if !ok {
		return "", false
	}
	to, ok := st["to"].(string)
	return to, ok
}

// reportTaskFor is the Create Record step of "Create Incident Report Task".
// Service, assignment group, incident and assignee come from the incident;
// priority is fixed at 1 - Critical and type at Incident Report.
func reportTaskFor(src repository.IncidentReportSource) repository.NewIncidentReportTask {
	return repository.NewIncidentReportTask{
		IncidentID:        src.IncidentID,
		Subject:           "[Incident Report] Create the incident report for " + src.Number,
		ServiceID:         src.ServiceID,
		AssignmentGroupID: src.AssignmentGroupID,
		AssignedToID:      src.AssignedToID,
		CreatedBy:         incidentReportActor,
	}
}

// incidentPriorityLabels are ServiceNow's display values for incident.priority,
// which is what the flow's data pill rendered into the report.
var incidentPriorityLabels = map[string]string{
	"CRITICAL": "1 - Critical",
	"HIGH":     "2 - High",
	"MODERATE": "3 - Moderate",
	"LOW":      "4 - Low",
	"PLANNING": "5 - Planning",
}

// incidentReportSections are the template's headings in order. The first
// three are filled from the incident; the rest are "-" placeholders for
// whoever writes the report up.
var incidentReportSections = []string{
	"Incident Number",
	"Incident Severity Level",
	"Incident Identification Time",
	"Timeline",
	"Affected Users or Customers",
	"Affected Functionality",
	"Cause(s) if known",
	"Initial Response Actions Taken",
	"Next Steps",
}

// renderIncidentReport is the Update Record step of "Incident Report
// Generator", reproducing what ServiceNow actually STORED, not the flow
// definition's markup. The one Generator-written report on staging
// (INC0015592, 2024-11-07, the week the flow was active) shows ServiceNow's
// editor drops the definition's font-size spans and data-tinymcerootblock
// attribute and keeps a trailing break after the last value; identification
// time is the incident's creation time in UTC, unlabelled. That stored text
// is the golden case in TestIncidentReport_Template.
func renderIncidentReport(src repository.IncidentReportSource) string {
	priority := "-"
	if src.Priority != nil {
		if label, ok := incidentPriorityLabels[*src.Priority]; ok {
			priority = label
		} else {
			priority = *src.Priority
		}
	}
	values := []string{
		src.Number,
		priority,
		src.CreatedOn.UTC().Format("2006-01-02 15:04:05"),
	}

	var b strings.Builder
	b.WriteString(`<p>`)
	for i, heading := range incidentReportSections {
		b.WriteString(`<strong>`)
		b.WriteString(html.EscapeString(heading))
		b.WriteString(`</strong><br />`)
		value := "-"
		if i < len(values) && values[i] != "" {
			value = values[i]
		}
		b.WriteString(html.EscapeString(value))
		b.WriteString(`<br /><br />`)
	}
	b.WriteString(`</p>`)
	return b.String()
}

// IncidentReportDrainer polls event_outbox for incident changes and runs
// IncidentReportService on each, one transaction per row.
//
// Same trigger-and-outbox shape as CRNoticeDrainer and CloudStatusDrainer,
// with one difference: a row is marked done in the transaction that applied
// its effect, not when it is claimed. A crash or failed write leaves it to be
// retried with backoff; a row that keeps failing is parked after MaxAttempts
// with its error on event_outbox.last_error.
type IncidentReportDrainer struct {
	Repo        repository.IncidentReportRepository
	Service     IncidentReportService
	Interval    time.Duration
	MaxAttempts int
	// SchemaRecheck is how often to look again for migration 0181 while it
	// is missing (default incidentReportSchemaRecheck).
	SchemaRecheck time.Duration
}

const (
	// incidentReportSchemaRecheck paces the idle check while migration 0181
	// is missing; incidentReportSchemaReminder is how often that state is
	// logged again after the first time.
	incidentReportSchemaRecheck  = 5 * time.Minute
	incidentReportSchemaReminder = time.Hour
)

// NewIncidentReportDrainer constructs the poller.
func NewIncidentReportDrainer(repo repository.IncidentReportRepository, svc IncidentReportService, interval time.Duration, maxAttempts int) *IncidentReportDrainer {
	return &IncidentReportDrainer{Repo: repo, Service: svc, Interval: interval, MaxAttempts: maxAttempts}
}

// Run drains until ctx is cancelled. A batch that was full and fully applied
// is followed immediately by the next, so a backlog drains at full speed; any
// other pass waits the interval, which is also what spaces out retries of a
// failing row.
func (d *IncidentReportDrainer) Run(ctx context.Context) {
	slog.InfoContext(ctx, "incidentreport: drainer started", "interval", d.Interval.String())
	if !d.waitForSchema(ctx) {
		slog.InfoContext(ctx, "incidentreport: drainer stopped")
		return
	}
	for {
		n, err := d.drainOnce(ctx)
		if ctx.Err() != nil {
			slog.InfoContext(ctx, "incidentreport: drainer stopped")
			return
		}
		if err != nil && isSchemaFault(err) {
			// The schema went away under a running drainer (a table
			// recreated, a migration rolled back): go back to waiting
			// rather than failing every poll.
			if !d.waitForSchema(ctx) {
				slog.InfoContext(ctx, "incidentreport: drainer stopped")
				return
			}
			continue
		}
		if err != nil {
			// Keep polling: a transient database error must not take the
			// drainer down for the life of the process.
			slog.ErrorContext(ctx, "incidentreport: drain failed", "err", err)
		}
		if n == incidentReportBatchSize && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "incidentreport: drainer stopped")
			return
		case <-time.After(d.Interval):
		}
	}
}

// waitForSchema blocks until migration 0181 is present, returning false only
// if ctx ends first. While it is missing it logs once, then once an hour, and
// checks again every SchemaRecheck: deploying the code before the migration
// idles the flows quietly and they start by themselves once it is applied --
// no restart, no error every poll.
func (d *IncidentReportDrainer) waitForSchema(ctx context.Context) bool {
	recheck := d.SchemaRecheck
	if recheck <= 0 {
		recheck = incidentReportSchemaRecheck
	}
	var lastLogged time.Time
	waited := false
	for {
		missing, err := d.Repo.MissingSchema(ctx)
		switch {
		case err == nil && len(missing) == 0:
			if waited {
				slog.InfoContext(ctx, "incidentreport: migration 0181 is present, incident report flows starting")
			}
			return true
		case lastLogged.IsZero() || time.Since(lastLogged) >= incidentReportSchemaReminder:
			if err != nil {
				slog.ErrorContext(ctx, "incidentreport: cannot check for migration 0181; incident report flows idle", "err", err)
			} else {
				slog.ErrorContext(ctx, "incidentreport: migration 0181 not applied; incident report flows idle until it is",
					"missing", missing, "recheck", recheck.String())
			}
			lastLogged = time.Now()
		}
		waited = true
		select {
		case <-ctx.Done():
			return false
		case <-time.After(recheck):
		}
	}
}

// isSchemaFault reports whether err is a missing table, column or function
// -- the database lacking this code's migration, not a bad row.
func isSchemaFault(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "42703", "42P01", "42883": // undefined_column, undefined_table, undefined_function
		return true
	}
	return false
}

// drainOnce processes one batch and returns how many rows it applied.
func (d *IncidentReportDrainer) drainOnce(ctx context.Context) (int, error) {
	ids, err := d.Repo.PendingChanges(ctx, incidentReportBatchSize)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			return applied, nil
		}
		// One row's failure must not hold up the rest: rows are
		// independent, and a failed one stays pending for the next pass.
		ok, err := d.Repo.ProcessChange(ctx, id, d.Service.HandleChange)
		if ok {
			applied++
		}
		if err != nil {
			parked, recErr := d.Repo.RecordFailure(ctx, id, err.Error(), d.MaxAttempts)
			switch {
			case recErr != nil:
				slog.ErrorContext(ctx, "incidentreport: change failed and recording the failure also failed",
					"outboxId", id, "err", err, "recordErr", recErr)
			case parked:
				slog.ErrorContext(ctx, "incidentreport: change failed too many times, parked",
					"outboxId", id, "maxAttempts", d.MaxAttempts, "err", err)
			default:
				slog.WarnContext(ctx, "incidentreport: change failed, will retry",
					"outboxId", id, "err", err)
			}
		}
	}
	return applied, nil
}

// ---------------------------------------------------------------------------
// "[WSO2 Cloud Ops] Post resolution tasks"
//
// ServiceNow: record_update incident, run as system, condition
// business_service=Choreo ^OR business_service=Asgardeo ^ stateCHANGESTO6
// (Resolved). Its blocks, in order, each If independent of the others
// (discovery script 55, 2026-10-05):
//
//	 1 Log "Incident # - <number> - is resolved"
//	 2 If close_code = False Alarm          -> 3 alert task, P1, WSO2 SRE Team
//	 4 If close_code = Duplicate            -> 5 alert task, P1, WSO2 SRE Team
//	 6 If close_code = Not Actionable Alert -> 7 alert task, P2, WSO2 SRE Team
//	 8 If close_code = Solved (Work Around) and problem_id is empty
//	      9 create problem; 10 incident.problem_id = it
//	     11 If Choreo       -> 12 problem group Choreo Special Ops
//	     13 Else If Asgardeo -> 14 problem group Asgardeo Operations Team
//	15 If u_runbook_solve_the_issue = 2 and close_code != Solved (Work Around)
//	     -> 16 runbook task
//
// Blocks 15-16 are NOT ported: incident.u_runbook_solve_the_issue has no
// Postgres column and the portal has no field for it, so the condition can
// never be true here. See the PR for what porting it needs.
//
// Blocks 9 and 12/14 are one insert here (the problem is created with its
// group) rather than a create and an update; the stored result is the same.
// ---------------------------------------------------------------------------

// The two services the flow's trigger names, and the groups it sets, as
// Postgres ids (sysidToUUID of the ServiceNow sys_ids in the flow).
const (
	postResolutionServiceChoreo   = "b9c999f8-1b86-a010-00ae-86acdd4bcb61"
	postResolutionServiceAsgardeo = "97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"

	groupWSO2SRETeam            = "f991f369-1b88-b410-cb68-98aebd4bcb13"
	groupChoreoSpecialOps       = "fe0d8868-1b0b-3010-d64e-64a2604bcb3c"
	groupAsgardeoOperationsTeam = "e66e38f7-870b-b110-c049-76e4dabb35aa"
)

// postResolutionTasks runs the flow's blocks 1-14 for an incident that has
// just changed to Resolved. src is the incident as it is now, which is what
// the flow's {{Updated_1.current}} pills read.
func postResolutionTasks(ctx context.Context, tx repository.IncidentReportTx, src repository.IncidentReportSource) error {
	service := strOrEmpty(src.ServiceID)
	if service != postResolutionServiceChoreo && service != postResolutionServiceAsgardeo {
		return nil
	}
	slog.InfoContext(ctx, "Incident # - "+src.Number+" - is resolved", "incidentId", src.IncidentID)

	code := strOrEmpty(src.ResolutionCode)
	for _, t := range alertTasksFor(src, code) {
		id, number, err := tx.CreateIncidentTask(ctx, t)
		if err != nil {
			return err
		}
		slog.InfoContext(ctx, "postresolution: created alert task",
			"incidentId", src.IncidentID, "taskId", id, "taskNumber", number)
	}

	if code == "SOLVED_WORK_AROUND" && strOrEmpty(src.ProblemID) == "" {
		id, number, err := tx.CreateProblem(ctx, problemFor(src))
		if err != nil {
			return err
		}
		if err := tx.LinkProblem(ctx, src.IncidentID, id, incidentReportActor); err != nil {
			return err
		}
		slog.InfoContext(ctx, "postresolution: created problem",
			"incidentId", src.IncidentID, "problemId", id, "problemNumber", number)
	}
	return nil
}

// alertTasksFor is blocks 2-7. close_code is a single value, so at most one
// matches. ServiceNow's "Duplicate" (label "Duplicate Alert") arrives in
// Postgres as DUPLICATE_ALERT from the sync and as DUPLICATE from the
// portal's resolve; both are that one choice. Subjects are ServiceNow's,
// including its "Falser Alarm".
func alertTasksFor(src repository.IncidentReportSource, code string) []repository.NewIncidentTask {
	var subject, priority string
	switch code {
	case "FALSE_ALARM":
		subject, priority = "[Alert Task][Falser Alarm] "+src.Number+" alert is a false alarm", "CRITICAL"
	case "DUPLICATE", "DUPLICATE_ALERT":
		subject, priority = "[Alert Task][Duplicate Alert] "+src.Number+" alert is a duplicate", "CRITICAL"
	case "NOT_ACTIONABLE_ALERT":
		subject, priority = "[Alert Task][Not Actionable Alert] "+src.Number+" is not an actionable alert", "HIGH"
	default:
		return nil
	}
	group := groupWSO2SRETeam
	return []repository.NewIncidentTask{{
		IncidentID:        src.IncidentID,
		Subject:           subject,
		Priority:          priority,
		ServiceID:         src.ServiceID,
		AssignmentGroupID: &group,
		CreatedBy:         incidentReportActor,
	}}
}

// problemFor is blocks 9 and 11-14: service, impact and urgency copied from
// the incident, the incident linked, and the group chosen by which of the two
// services the incident is on. The flow's If compares a transform of the
// incident to CHOREO / ASGARDEO; the trigger admits only those two services,
// so the service decides it.
//
// The flow also copies the incident's priority, but ServiceNow's "Priority
// Problem Lookup" runs on the insert and overwrites it from impact x urgency
// (always_replace; discovery script 64) -- so the priority is derived here
// too, which also covers an incident whose own priority is missing. An
// absent impact or urgency is ServiceNow's problem default, 3 - Low.
func problemFor(src repository.IncidentReportSource) repository.NewIncidentProblem {
	group := groupAsgardeoOperationsTeam
	if strOrEmpty(src.ServiceID) == postResolutionServiceChoreo {
		group = groupChoreoSpecialOps
	}
	impact, urgency := strOrDefault(src.Impact, "LOW"), strOrDefault(src.Urgency, "LOW")
	priority := priorityFromImpactUrgency(impact, urgency)
	return repository.NewIncidentProblem{
		IncidentID:        src.IncidentID,
		Subject:           "Fix the root cause of " + src.Number,
		ServiceID:         src.ServiceID,
		Priority:          &priority,
		Impact:            &impact,
		Urgency:           &urgency,
		AssignmentGroupID: &group,
		CreatedBy:         incidentReportActor,
	}
}

// strOrDefault is *s, or def for nil or "".
func strOrDefault(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

// strOrEmpty is *s, or "" for nil.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
