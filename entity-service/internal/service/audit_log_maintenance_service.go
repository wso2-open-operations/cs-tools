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
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AuditLogMaintenanceService runs the audit log's upkeep. It is called once a
// day by csm-scheduled-tasks (its audit_log_maintenance sub-cron), which also
// retries a failed run and sends the failure mail.
type AuditLogMaintenanceService interface {
	// RunMaintenance creates the coming monthly partitions, checks the log's
	// health, puts back audit triggers that went missing and deletes what is past
	// the retention period (old partitions, and old rows in the catch-all). A
	// deletion that fails is reported as a step error. It always tries
	// every step and returns what it found, including a step that could not run
	// (StepErrors), so one failing step never hides the others' results. The
	// error is set only when the request itself was cancelled.
	RunMaintenance(ctx context.Context) (domain.AuditLogMaintenanceResponse, error)
}

type auditLogMaintenanceService struct {
	repo            repository.AuditLogRepository
	monthsAhead     int
	retentionMonths int
}

// NewAuditLogMaintenanceService constructs the service. monthsAhead is how many
// months past the current one get a partition; retentionMonths is how long audit
// rows are kept before their partition is deleted.
func NewAuditLogMaintenanceService(repo repository.AuditLogRepository, monthsAhead, retentionMonths int) AuditLogMaintenanceService {
	return &auditLogMaintenanceService{repo: repo, monthsAhead: monthsAhead, retentionMonths: retentionMonths}
}

// RunMaintenance implements AuditLogMaintenanceService.
func (s *auditLogMaintenanceService) RunMaintenance(ctx context.Context) (domain.AuditLogMaintenanceResponse, error) {
	// Empty slices, not nil, so the JSON shows [] and not null.
	resp := domain.AuditLogMaintenanceResponse{
		PartitionsRetired:  []string{},
		HealthProblems:     []string{},
		RepairedTriggers:   []domain.AuditTriggerProblem{},
		UnresolvedTriggers: []domain.AuditTriggerProblem{},
		StepErrors:         []string{},
	}

	created, err := s.repo.CreatePartitions(ctx, s.monthsAhead)
	if err != nil {
		resp.StepErrors = append(resp.StepErrors, s.stepFailed(ctx, "could not create partitions", err))
	}
	resp.PartitionsCreated = created

	repaired, unresolved, err := s.reconcileTriggers(ctx)
	if err != nil {
		resp.StepErrors = append(resp.StepErrors, s.stepFailed(ctx, "could not reconcile triggers", err))
	}
	resp.RepairedTriggers = append(resp.RepairedTriggers, repaired...)
	resp.UnresolvedTriggers = append(resp.UnresolvedTriggers, unresolved...)

	// Last, so a failure here never hides what the steps above found. Dropping
	// is irreversible, so the names are logged.
	retired, err := s.repo.RetirePartitions(ctx, s.retentionMonths)
	if err != nil {
		resp.StepErrors = append(resp.StepErrors, s.stepFailed(ctx, "could not delete expired partitions", err))
	}
	resp.PartitionsRetired = append(resp.PartitionsRetired, retired...)
	if len(retired) > 0 {
		slog.InfoContext(ctx, "audit log: deleted expired partitions", "partitions", retired, "retentionMonths", s.retentionMonths)
	}

	// The catch-all partition is outside the partition ranges above, so its
	// expired rows are deleted on their own.
	purged, err := s.repo.PurgeDefaultPartition(ctx, s.retentionMonths)
	if err != nil {
		resp.StepErrors = append(resp.StepErrors, s.stepFailed(ctx, "could not delete expired rows from the catch-all partition", err))
	}
	resp.DefaultRowsDeleted = purged
	if purged > 0 {
		slog.InfoContext(ctx, "audit log: deleted expired rows from the catch-all partition", "rows", purged, "retentionMonths", s.retentionMonths)
	}

	// Read-only, and last so it reports the state this run leaves: a partition
	// created above counts, and so does one it failed to create.
	healthProblems, err := s.repo.PartitionHealth(ctx, s.monthsAhead)
	if err != nil {
		resp.StepErrors = append(resp.StepErrors, s.stepFailed(ctx, "could not check partition health", err))
	}
	resp.HealthProblems = append(resp.HealthProblems, healthProblems...)

	slog.InfoContext(ctx, "audit log: maintenance run finished",
		"partitionsCreated", resp.PartitionsCreated, "partitionsRetired", len(resp.PartitionsRetired), "defaultRowsDeleted", resp.DefaultRowsDeleted, "healthProblems", len(resp.HealthProblems),
		"repairedTriggers", len(resp.RepairedTriggers), "unresolvedTriggers", len(resp.UnresolvedTriggers),
		"failedSteps", len(resp.StepErrors))
	return resp, ctx.Err()
}

// stepFailed logs a step's full error and returns the short, detail-free text
// that goes in the response.
func (s *auditLogMaintenanceService) stepFailed(ctx context.Context, message string, err error) string {
	slog.ErrorContext(ctx, "audit log: maintenance step failed", "step", message, "err", err)
	return message
}

// reconcileTriggers repairs what it can. It reconciles twice: the first call
// repairs and lists everything it found, the second lists what is still wrong.
// A trigger only the first call listed was repaired.
func (s *auditLogMaintenanceService) reconcileTriggers(ctx context.Context) (repaired, unresolved []domain.AuditTriggerProblem, err error) {
	found, err := s.repo.Reconcile(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	unresolved, err = s.repo.Reconcile(ctx, false)
	if err != nil {
		// The repair already ran and the caller only gets the error, so say which
		// tables it touched.
		slog.WarnContext(ctx, "audit log: could not verify the trigger repair", "repairCandidates", tableNames(found))
		return nil, nil, err
	}

	stillBroken := make(map[string]bool, len(unresolved))
	for _, p := range unresolved {
		stillBroken[p.Schema+"."+p.Table] = true
	}
	for _, p := range found {
		if !stillBroken[p.Schema+"."+p.Table] {
			repaired = append(repaired, p)
		}
	}
	return repaired, unresolved, nil
}

// tableNames lists the tables in problems as schema.table.
func tableNames(problems []domain.AuditTriggerProblem) []string {
	names := make([]string, 0, len(problems))
	for _, p := range problems {
		names = append(names, p.Schema+"."+p.Table)
	}
	return names
}
