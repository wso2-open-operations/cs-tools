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
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeAuditLogRepo records the calls the service makes, in order.
type fakeAuditLogRepo struct {
	calls []string

	created         int
	createErr       error
	healthProblems  []string
	repairedFound   []domain.AuditTriggerProblem // returned when repair is on
	stillUnresolved []domain.AuditTriggerProblem // returned when repair is off
	reconcileErr    error
	verifyErr       error // returned only by the second (repair off) reconcile
}

func (f *fakeAuditLogRepo) CreatePartitions(_ context.Context, monthsAhead int) (int, error) {
	f.calls = append(f.calls, fmt.Sprintf("create(%d)", monthsAhead))
	return f.created, f.createErr
}

func (f *fakeAuditLogRepo) PartitionHealth(context.Context) ([]string, error) {
	f.calls = append(f.calls, "health")
	return f.healthProblems, nil
}

func (f *fakeAuditLogRepo) Reconcile(_ context.Context, repair bool) ([]domain.AuditTriggerProblem, error) {
	f.calls = append(f.calls, fmt.Sprintf("reconcile(%t)", repair))
	if f.reconcileErr != nil {
		return nil, f.reconcileErr
	}
	if repair {
		return f.repairedFound, nil
	}
	if f.verifyErr != nil {
		return nil, f.verifyErr
	}
	return f.stillUnresolved, nil
}

func triggerProblem(table, text string) domain.AuditTriggerProblem {
	return domain.AuditTriggerProblem{Schema: "public", Table: table, Problem: text}
}

func TestAuditLogMaintenance_RunsEveryStepInOrder(t *testing.T) {
	repo := &fakeAuditLogRepo{created: 1}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}

	want := "create(3) health reconcile(true) reconcile(false)"
	if got := strings.Join(repo.calls, " "); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if resp.PartitionsCreated != 1 {
		t.Errorf("PartitionsCreated = %d, want 1", resp.PartitionsCreated)
	}
}

func TestAuditLogMaintenance_NothingWrongGivesEmptyLists(t *testing.T) {
	resp, err := NewAuditLogMaintenanceService(&fakeAuditLogRepo{}, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}

	if resp.HealthProblems == nil || resp.RepairedTriggers == nil || resp.UnresolvedTriggers == nil || resp.StepErrors == nil {
		t.Error("lists must be empty, not nil, so the JSON shows [] and not null")
	}
	if len(resp.HealthProblems)+len(resp.RepairedTriggers)+len(resp.UnresolvedTriggers) != 0 {
		t.Errorf("expected no problems, got %+v", resp)
	}
}

func TestAuditLogMaintenance_ReportsHealthProblems(t *testing.T) {
	repo := &fakeAuditLogRepo{healthProblems: []string{"rows landed in the catch-all partition"}}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if len(resp.HealthProblems) != 1 {
		t.Errorf("HealthProblems = %v, want one", resp.HealthProblems)
	}
}

func TestAuditLogMaintenance_FailedStepIsReportedAndDoesNotStopTheOthers(t *testing.T) {
	repo := &fakeAuditLogRepo{createErr: errors.New("pq: password authentication failed for user secret_user"), healthProblems: []string{"too few future months"}}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("a failed step must be reported in the response, not as an error: %v", err)
	}

	want := "create(3) health reconcile(true) reconcile(false)"
	if got := strings.Join(repo.calls, " "); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if len(resp.StepErrors) != 1 || len(resp.HealthProblems) != 1 {
		t.Errorf("stepErrors = %v, healthProblems = %v; want the failure and the later step's result both kept", resp.StepErrors, resp.HealthProblems)
	}
	if strings.Contains(resp.StepErrors[0], "secret_user") || strings.Contains(resp.StepErrors[0], "password") {
		t.Errorf("step error leaks database detail: %q", resp.StepErrors[0])
	}
}

func TestAuditLogMaintenance_RepairedTriggerIsNotUnresolved(t *testing.T) {
	repo := &fakeAuditLogRepo{repairedFound: []domain.AuditTriggerProblem{triggerProblem("project", "registered table has no audit trigger")}}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if len(resp.RepairedTriggers) != 1 || len(resp.UnresolvedTriggers) != 0 {
		t.Errorf("repaired = %v, unresolved = %v; want one repaired and none unresolved", resp.RepairedTriggers, resp.UnresolvedTriggers)
	}
}

func TestAuditLogMaintenance_StillBrokenTriggerIsUnresolvedNotRepaired(t *testing.T) {
	broken := triggerProblem("project", "registered table was renamed or dropped")
	repo := &fakeAuditLogRepo{
		repairedFound:   []domain.AuditTriggerProblem{broken},
		stillUnresolved: []domain.AuditTriggerProblem{broken},
	}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if len(resp.UnresolvedTriggers) != 1 || len(resp.RepairedTriggers) != 0 {
		t.Errorf("repaired = %v, unresolved = %v; want one unresolved and none repaired", resp.RepairedTriggers, resp.UnresolvedTriggers)
	}
}

func TestAuditLogMaintenance_ReconcileErrorIsReportedAsAStepError(t *testing.T) {
	repo := &fakeAuditLogRepo{reconcileErr: errors.New("boom")}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if len(resp.StepErrors) != 1 {
		t.Errorf("stepErrors = %v, want one", resp.StepErrors)
	}
}

func TestAuditLogMaintenance_NothingWrongHasNoStepErrors(t *testing.T) {
	resp, err := NewAuditLogMaintenanceService(&fakeAuditLogRepo{}, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if resp.StepErrors == nil || len(resp.StepErrors) != 0 {
		t.Errorf("stepErrors = %v, want an empty list", resp.StepErrors)
	}
}

func TestAuditLogMaintenance_CancelledRequestIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewAuditLogMaintenanceService(&fakeAuditLogRepo{}, 3).RunMaintenance(ctx); err == nil {
		t.Fatal("a cancelled request should not be answered as a success")
	}
}

func TestAuditLogMaintenance_FailedVerificationLogsWhatTheRepairTouched(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	repo := &fakeAuditLogRepo{
		repairedFound: []domain.AuditTriggerProblem{triggerProblem("account", "registered table has no audit trigger")},
		verifyErr:     errors.New("connection lost"),
	}

	resp, err := NewAuditLogMaintenanceService(repo, 3).RunMaintenance(context.Background())
	if err != nil {
		t.Fatalf("RunMaintenance: %v", err)
	}
	if len(resp.StepErrors) != 1 || resp.StepErrors[0] != "could not reconcile triggers" {
		t.Errorf("StepErrors = %v, want the reconcile step reported", resp.StepErrors)
	}
	if got := logs.String(); !strings.Contains(got, "public.account") {
		t.Errorf("log does not name the repaired table:\n%s", got)
	}
}
