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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const transferCaseID = "5b1b1c1e-0000-4000-8000-000000000001"

func transferStr(s string) *string { return &s }

func transferSev(s domain.CaseSeverity) *domain.CaseSeverity { return &s }

func transferIssue(i domain.CaseIssueType) *domain.CaseIssueType { return &i }

func transferState(s domain.CaseState) *domain.CaseState { return &s }

// transferStubRepo is a CaseRepository (through the embedded stub, whose
// unconfigured methods panic) that also implements the transfer repository,
// recording what the service asked it to do and running the remote step the way
// the real one does: after its own work, before it "commits".
type transferStubRepo struct {
	*stubCaseRepo
	plans    []repository.CaseTypeTransfer
	result   repository.CaseTypeTransferResult
	err      error // the repository's own failure, returned after the remote step succeeded
	remoteOK bool  // set once the remote step has returned without an error
	activity []string
}

func (r *transferStubRepo) TransferCaseType(ctx context.Context, plan repository.CaseTypeTransfer, remote repository.CaseTypeTransferRemote) (repository.CaseTypeTransferResult, error) {
	r.plans = append(r.plans, plan)
	var remoteResult *repository.CaseTypeTransferRemoteResult
	if remote != nil {
		var err error
		remoteResult, err = remote(ctx, repository.CaseTypeTransferBefore{PreviousType: r.result.PreviousType, PreviousSeverity: r.result.PreviousSeverity})
		if err != nil {
			// Same contract as the real repository: the remote step failing rolls
			// everything back and is what the caller sees.
			return repository.CaseTypeTransferResult{}, err
		}
		r.remoteOK = true
	}
	if r.err != nil {
		return repository.CaseTypeTransferResult{}, r.err
	}
	out := r.result
	if out.Type == "" {
		out.Type = plan.TargetType
	}
	if remoteResult != nil && remoteResult.State != nil {
		out.State = remoteResult.State
	}
	return out, nil
}

// transferMirror is the ServiceNow-backed CaseService of the dual-write mode
// with only UpdateCase implemented.
type transferMirror struct {
	CaseService
	calls []domain.UpdateCaseRequest
	resp  domain.UpdateCaseResponse
	err   error
	// onUpdate runs inside UpdateCase, e.g. to cancel the request as a timeout would.
	onUpdate func()
	// getCase answers the re-check after an ambiguous failure; getCalls counts them.
	getCase  func(ctx context.Context, id string) (domain.CaseView, error)
	getCalls int
}

func (m *transferMirror) UpdateCase(_ context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	m.calls = append(m.calls, req)
	if m.onUpdate != nil {
		m.onUpdate()
	}
	return m.resp, m.err
}

func (m *transferMirror) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	m.getCalls++
	if m.getCase == nil {
		return domain.CaseView{}, errors.New("not configured")
	}
	return m.getCase(ctx, id)
}

func newTransferStubRepo(result repository.CaseTypeTransferResult) *transferStubRepo {
	r := &transferStubRepo{stubCaseRepo: &stubCaseRepo{}, result: result}
	r.stubCaseRepo.recordCaseFieldChangeActivity = func(_ context.Context, caseID, field, oldValue, newValue, actorEmail string) error {
		r.activity = append(r.activity, strings.Join([]string{caseID, field, oldValue, newValue, actorEmail}, "|"))
		return nil
	}
	return r
}

func transferUsers() stubUserRepo {
	return stubUserRepo{getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
		return domain.User{ID: "u-1", Email: email}, nil
	}}
}

func engagementTransfer() domain.UpdateCaseRequest {
	et, pt := domain.EngagementTypeMigration, domain.EngagementPaymentTypeFOC
	return domain.UpdateCaseRequest{ID: transferCaseID, Type: transferStr("engagement"), EngagementType: &et, EngagementPaymentType: &pt}
}

func caseTransfer(sev domain.CaseSeverity) domain.UpdateCaseRequest {
	return domain.UpdateCaseRequest{ID: transferCaseID, Type: transferStr("case"), Severity: transferSev(sev), IssueType: transferIssue(domain.CaseIssueTypeError)}
}

func TestValidateCaseTypeTransfer(t *testing.T) {
	et, pt := domain.EngagementTypeMigration, domain.EngagementPaymentTypeFOC
	badET := domain.EngagementType("bogus")
	vars := []domain.Variable{{ID: "0c1c1c1e-0000-4000-8000-000000000009", Value: "x"}}
	catalog, item := "0c1c1c1e-0000-4000-8000-000000000007", "0c1c1c1e-0000-4000-8000-000000000008"

	tests := []struct {
		name       string
		req        domain.UpdateCaseRequest
		wantTarget string
		wantErr    string // substring; "" = valid
	}{
		{name: "engagement, migration + FOC", req: engagementTransfer(), wantTarget: "engagement"},
		{name: "security report analysis takes no companion", req: domain.UpdateCaseRequest{Type: transferStr("security_report_analysis")}, wantTarget: "security_report_analysis"},
		{
			name:       "service request with a catalog item and an answer",
			req:        domain.UpdateCaseRequest{Type: transferStr("service_request"), CatalogID: &catalog, CatalogItemID: &item, Variables: vars},
			wantTarget: "service_request",
		},
		// Incident (S0-S3) and Query (S4) are both a "case"; the severity picks which.
		{name: "case at S4 (a Query)", req: caseTransfer(domain.CaseSeverityLow), wantTarget: "case"},
		{name: "case at S3 (an Incident)", req: caseTransfer(domain.CaseSeverityMedium), wantTarget: "case"},
		{name: "case at S0 (an Incident)", req: caseTransfer(domain.CaseSeverityCatastrophic), wantTarget: "case"},
		{name: "the default_case alias is a case", req: func() domain.UpdateCaseRequest {
			r := caseTransfer(domain.CaseSeverityHigh)
			r.Type = transferStr("default_case")
			return r
		}(), wantTarget: "case"},

		{name: "no type", req: domain.UpdateCaseRequest{}, wantErr: "type is required"},
		{name: "unknown type", req: domain.UpdateCaseRequest{Type: transferStr("incident")}, wantErr: "invalid value"},
		{name: "announcement is system-managed", req: domain.UpdateCaseRequest{Type: transferStr("announcement")}, wantErr: "system-managed"},
		{
			name: "type cannot ride with a state change",
			req: func() domain.UpdateCaseRequest {
				r := engagementTransfer()
				s := domain.CaseStateClosed
				r.State = &s
				return r
			}(),
			wantErr: "cannot be combined",
		},
		{name: "engagement needs an engagement type", req: domain.UpdateCaseRequest{Type: transferStr("engagement"), EngagementPaymentType: &pt}, wantErr: "engagementType is required"},
		{name: "engagement needs a payment type", req: domain.UpdateCaseRequest{Type: transferStr("engagement"), EngagementType: &et}, wantErr: "engagementPaymentType is required"},
		{name: "engagement type must be known", req: domain.UpdateCaseRequest{Type: transferStr("engagement"), EngagementType: &badET, EngagementPaymentType: &pt}, wantErr: "engagementType contains invalid value"},
		{
			name: "engagement takes no severity",
			req: func() domain.UpdateCaseRequest {
				r := engagementTransfer()
				r.Severity = transferSev(domain.CaseSeverityLow)
				return r
			}(),
			wantErr: "severity may only accompany",
		},
		{
			name: "engagement takes no issue type",
			req: func() domain.UpdateCaseRequest {
				r := engagementTransfer()
				r.IssueType = transferIssue(domain.CaseIssueTypeError)
				return r
			}(),
			wantErr: "issueType is only accepted",
		},
		{name: "case needs a severity", req: domain.UpdateCaseRequest{Type: transferStr("case"), IssueType: transferIssue(domain.CaseIssueTypeError)}, wantErr: "severity is required"},
		{name: "case needs an issue type", req: domain.UpdateCaseRequest{Type: transferStr("case"), Severity: transferSev(domain.CaseSeverityLow)}, wantErr: "issueType is required"},
		{
			name: "case takes no engagement fields",
			req: func() domain.UpdateCaseRequest {
				r := caseTransfer(domain.CaseSeverityLow)
				r.EngagementType = &et
				return r
			}(),
			wantErr: "only accepted when type is \"engagement\"",
		},
		{name: "case severity must be known", req: caseTransfer("urgent"), wantErr: "severity contains invalid value"},
		{
			name:    "service request needs a catalog",
			req:     domain.UpdateCaseRequest{Type: transferStr("service_request"), Variables: vars},
			wantErr: "catalogId and catalogItemId are required",
		},
		{
			name:    "service request needs an answer",
			req:     domain.UpdateCaseRequest{Type: transferStr("service_request"), CatalogID: &catalog, CatalogItemID: &item},
			wantErr: "variables must contain at least one entry",
		},
		{
			name:    "service request catalog ids must be uuids",
			req:     domain.UpdateCaseRequest{Type: transferStr("service_request"), CatalogID: transferStr("nope"), CatalogItemID: &item, Variables: vars},
			wantErr: "catalogId",
		},
		{
			name:    "security report analysis takes no severity",
			req:     domain.UpdateCaseRequest{Type: transferStr("security_report_analysis"), Severity: transferSev(domain.CaseSeverityLow)},
			wantErr: "severity may only accompany",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCaseTypeTransfer(tc.req)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.wantTarget {
					t.Errorf("target = %q, want %q", got, tc.wantTarget)
				}
				return
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want a ValidationError containing %q, got %v", tc.wantErr, err)
			}
			if !strings.Contains(ve.Msg, tc.wantErr) {
				t.Errorf("message %q does not contain %q", ve.Msg, tc.wantErr)
			}
		})
	}
}

// Without the wiring a type change is refused exactly as it was before the
// transfer existed.
func TestCaseService_UpdateCase_TypeIsRefusedUntilTheTransferIsWired(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	_, err := svc.UpdateCase(context.Background(), engagementTransfer())
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "only supported for the ServiceNow data source") {
		t.Fatalf("want the original refusal, got %v", err)
	}
}

// A customer must not be able to re-type a case through whatever PATCH the
// customer portal's backend forwards.
func TestCaseService_UpdateCase_TypeTransferIsInternalOnly(t *testing.T) {
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
	svc := WithCaseTypeTransfer(NewCaseService(repo, transferUsers(), nil, restrictedAccess{}, nil), repo)

	_, err := svc.UpdateCase(context.Background(), engagementTransfer())
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("want a ForbiddenError, got %v", err)
	}
	if len(repo.plans) != 0 {
		t.Errorf("the repository was called for a caller who is not internal")
	}
}

// Plain Postgres: no remote step, the transfer is recorded in the activity feed
// and the response carries what the portal reads next.
func TestCaseService_UpdateCase_TransfersWithoutServiceNow(t *testing.T) {
	updatedOn := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{
		PreviousType: "case", UpdatedOn: updatedOn, State: transferState(domain.CaseStateWorkInProgress),
	})
	svc := WithCaseTypeTransfer(NewCaseService(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil), repo)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.UpdateCase(ctx, engagementTransfer())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.plans) != 1 {
		t.Fatalf("want 1 transfer, got %d", len(repo.plans))
	}
	plan := repo.plans[0]
	if plan.CaseID != transferCaseID || plan.TargetType != "engagement" || plan.ActorEmail != "jane.doe@example.com" {
		t.Errorf("unexpected plan: %+v", plan)
	}
	if plan.EngagementType == nil || *plan.EngagementType != domain.EngagementTypeMigration ||
		plan.EngagementPaymentType == nil || *plan.EngagementPaymentType != domain.EngagementPaymentTypeFOC {
		t.Errorf("engagement type/payment type not handed on: %+v", plan)
	}
	if resp.Case.Type != "engagement" || resp.Case.State == nil || *resp.Case.State != domain.CaseStateWorkInProgress || !resp.Case.UpdatedOn.Equal(updatedOn) {
		t.Errorf("unexpected response: %+v", resp.Case)
	}
	if len(repo.activity) != 1 || repo.activity[0] != transferCaseID+"|type|Case|Engagement|jane.doe@example.com" {
		t.Errorf("unexpected activity entries: %v", repo.activity)
	}
}

// Dual-write: ServiceNow is asked from inside the repository's transaction, with
// the canonical type, and the state it reports is what Postgres is given.
func TestCaseService_UpdateCase_AsksServiceNowInsideTheTransaction(t *testing.T) {
	snState := domain.CaseStateOpen
	mirror := &transferMirror{resp: domain.UpdateCaseResponse{
		Message: "from servicenow",
		Case:    domain.UpdatedCase{ID: transferCaseID, UpdatedBy: "sn-user", State: &snState},
	}}
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{
		PreviousType: "engagement", State: transferState(domain.CaseStateWorkInProgress), Severity: transferSev(domain.CaseSeverityLow),
	})
	svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, nil, "")
	svc = WithCaseTypeTransfer(svc, repo)

	req := caseTransfer(domain.CaseSeverityLow)
	req.Type = transferStr("default_case") // the portal's alias
	resp, err := svc.UpdateCase(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mirror.calls) != 1 {
		t.Fatalf("ServiceNow should be asked exactly once, got %d", len(mirror.calls))
	}
	if got := mirror.calls[0]; got.Type == nil || *got.Type != "case" || got.Severity == nil || *got.Severity != domain.CaseSeverityLow {
		t.Errorf("ServiceNow was not handed the canonical request: %+v", got)
	}
	if !repo.remoteOK {
		t.Errorf("the remote step did not run inside the repository transfer")
	}
	// ServiceNow's own receipt is kept, with the fields the portal reads next guaranteed.
	if resp.Message != "from servicenow" || resp.Case.Type != "case" || resp.Case.State == nil || *resp.Case.State != domain.CaseStateOpen {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.Case.Severity == nil || *resp.Case.Severity != domain.CaseSeverityLow {
		t.Errorf("severity should fall back to the transferred one, got %v", resp.Case.Severity)
	}
}

// ServiceNow refusing the transfer must undo the Postgres side and report
// nothing as done: no activity entry, no SLA change.
func TestCaseService_UpdateCase_ServiceNowRefusalLeavesNothingBehind(t *testing.T) {
	refusal := &apierror.ValidationError{Msg: "catalog item is not available"}
	mirror := &transferMirror{err: refusal}
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
	sla := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, sla, "")
	svc = WithCaseTypeTransfer(svc, repo)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	_, err := svc.UpdateCase(ctx, engagementTransfer())
	if !errors.Is(err, refusal) {
		t.Fatalf("want ServiceNow's refusal back, got %v", err)
	}
	if repo.remoteOK {
		t.Errorf("remote step reported success")
	}
	if len(repo.activity) != 0 {
		t.Errorf("an activity entry was written for a refused transfer: %v", repo.activity)
	}
	if len(sla.reviseCalls) != 0 {
		t.Errorf("SLA clocks were touched for a refused transfer: %+v", sla.reviseCalls)
	}
}

// ServiceNow moved the case but Postgres could not commit: the error reaches the
// caller (the case is not usable in the portal as it stands) and nothing after
// the commit runs.
func TestCaseService_UpdateCase_PostgresFailureAfterServiceNowIsReported(t *testing.T) {
	mirror := &transferMirror{resp: domain.UpdateCaseResponse{Case: domain.UpdatedCase{ID: transferCaseID}}}
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
	repo.err = &apierror.ConflictError{Msg: "this case has attachments"}
	sla := &fakeSLAEngineService{}
	svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, sla, "")
	svc = WithCaseTypeTransfer(svc, repo)

	_, err := svc.UpdateCase(context.Background(), engagementTransfer())
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want the repository's ConflictError, got %v", err)
	}
	if len(mirror.calls) != 1 {
		t.Errorf("ServiceNow should have been asked once before the Postgres failure, got %d", len(mirror.calls))
	}
	if len(sla.reviseCalls) != 0 || len(repo.activity) != 0 {
		t.Errorf("post-commit effects ran after a failed transfer: sla=%+v activity=%v", sla.reviseCalls, repo.activity)
	}
}

// The service never calls ServiceNow for a request it would reject anyway.
func TestCaseService_UpdateCase_InvalidTransferNeverReachesServiceNowOrPostgres(t *testing.T) {
	mirror := &transferMirror{}
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
	svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, nil, "")
	svc = WithCaseTypeTransfer(svc, repo)

	// A case without an issue type (a Query or an Incident alike).
	_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: transferCaseID, Type: transferStr("case"), Severity: transferSev(domain.CaseSeverityLow)})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want a ValidationError, got %v", err)
	}
	if len(mirror.calls) != 0 || len(repo.plans) != 0 {
		t.Errorf("an invalid request reached a backing store: mirror=%d repo=%d", len(mirror.calls), len(repo.plans))
	}
}

// S4 is a Query, S0-S3 an Incident, and both are a "case". Moving INTO "case"
// registers the clocks of the new severity (a Query only has the response
// clock; the policy resolver decides which), moving OUT of "case" cancels them,
// and a transfer between two other types has no clocks to touch.
func TestCaseService_UpdateCase_SLAClocksFollowTheCasesSeverity(t *testing.T) {
	tests := []struct {
		name         string
		previousType string
		req          domain.UpdateCaseRequest
		resultSev    *domain.CaseSeverity
		wantCalls    int
		wantSeverity *domain.CaseSeverity // nil = cancel only
	}{
		{name: "into a Query (S4)", previousType: "engagement", req: caseTransfer(domain.CaseSeverityLow), resultSev: transferSev(domain.CaseSeverityLow), wantCalls: 1, wantSeverity: transferSev(domain.CaseSeverityLow)},
		{name: "into an Incident (S1)", previousType: "service_request", req: caseTransfer(domain.CaseSeverityCritical), resultSev: transferSev(domain.CaseSeverityCritical), wantCalls: 1, wantSeverity: transferSev(domain.CaseSeverityCritical)},
		{name: "out of a case (Query or Incident alike)", previousType: "case", req: engagementTransfer(), wantCalls: 1, wantSeverity: nil},
		{name: "between two other types", previousType: "engagement", req: domain.UpdateCaseRequest{ID: transferCaseID, Type: transferStr("security_report_analysis")}, wantCalls: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: tc.previousType, Severity: tc.resultSev, ProjectID: "proj-1"})
			sla := &fakeSLAEngineService{}
			svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, &transferMirror{resp: domain.UpdateCaseResponse{Case: domain.UpdatedCase{ID: transferCaseID}}}, sla, "")
			svc = WithCaseTypeTransfer(svc, repo)

			if _, err := svc.UpdateCase(context.Background(), tc.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(sla.reviseCalls) != tc.wantCalls {
				t.Fatalf("ReviseCaseClocks calls = %d, want %d (%+v)", len(sla.reviseCalls), tc.wantCalls, sla.reviseCalls)
			}
			if tc.wantCalls == 0 {
				return
			}
			got := sla.reviseCalls[0]
			if got.caseID != transferCaseID || got.projectID != "proj-1" {
				t.Errorf("unexpected call: %+v", got)
			}
			switch {
			case tc.wantSeverity == nil && got.severity != nil:
				t.Errorf("clocks should be cancelled (nil severity), got %v", *got.severity)
			case tc.wantSeverity != nil && (got.severity == nil || *got.severity != *tc.wantSeverity):
				t.Errorf("severity = %v, want %v", got.severity, *tc.wantSeverity)
			}
		})
	}
}

// ServiceNow can fail in a way that does not say whether it applied the transfer:
// the request ran out of time, a connection dropped, a 5xx. Rolling Postgres back
// on a transfer ServiceNow DID apply would leave the case a different type in
// each store, so the service asks ServiceNow what it holds.
func TestCaseService_UpdateCase_ServiceNowTimeoutAfterApplyingIsCompleted(t *testing.T) {
	reqCtx, cancelRequest := context.WithCancel(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")))
	defer cancelRequest()
	snState := domain.CaseStateWorkInProgress
	var recheckSawCancelledRequest bool
	mirror := &transferMirror{
		err: context.DeadlineExceeded,
		// What a timeout does to the request: it is gone by the time the call returns.
		onUpdate: cancelRequest,
		getCase: func(ctx context.Context, _ string) (domain.CaseView, error) {
			recheckSawCancelledRequest = ctx.Err() != nil
			typ := "engagement"
			return domain.CaseView{Type: &typ, State: &snState}, nil
		},
	}
	committedOn := time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)
	ongoing := domain.CaseWorkStateOngoing
	repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case", UpdatedOn: committedOn, WorkState: &ongoing})
	svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, nil, "")
	svc = WithCaseTypeTransfer(svc, repo)

	resp, err := svc.UpdateCase(reqCtx, engagementTransfer())
	if err != nil {
		t.Fatalf("the transfer ServiceNow applied must complete, got %v", err)
	}
	if mirror.getCalls != 1 {
		t.Errorf("ServiceNow should be asked once what it holds, got %d", mirror.getCalls)
	}
	if recheckSawCancelledRequest {
		t.Errorf("the re-check ran on the cancelled request's context, so it could never succeed in the case it exists for")
	}
	if !repo.remoteOK {
		t.Errorf("the transfer was not carried through in Postgres")
	}
	if resp.Case.Type != "engagement" || resp.Case.State == nil || *resp.Case.State != domain.CaseStateWorkInProgress {
		t.Errorf("unexpected response: %+v", resp.Case)
	}
	if len(repo.activity) != 1 {
		t.Errorf("the completed transfer should be in the activity feed, got %v", repo.activity)
	}
	// ServiceNow gave no receipt here, so the response is built from what Postgres committed.
	if !resp.Case.UpdatedOn.Equal(committedOn) || resp.Case.UpdatedBy != "jane.doe@example.com" ||
		resp.Case.WorkState == nil || *resp.Case.WorkState != domain.CaseWorkStateOngoing || resp.Message == "" {
		t.Errorf("the response after a completed-after-timeout transfer is incomplete: %+v (message %q)", resp.Case, resp.Message)
	}
}

func TestCaseService_UpdateCase_ServiceNowErrorWithoutApplyingIsReported(t *testing.T) {
	for _, tc := range []struct {
		name    string
		getCase func(context.Context, string) (domain.CaseView, error)
	}{
		{"it still holds the old type", func(context.Context, string) (domain.CaseView, error) {
			typ := "case"
			return domain.CaseView{Type: &typ}, nil
		}},
		{"it cannot be asked", func(context.Context, string) (domain.CaseView, error) {
			return domain.CaseView{}, &apierror.ServiceUnavailableError{Msg: "down"}
		}},
		{"it answers without a type", func(context.Context, string) (domain.CaseView, error) { return domain.CaseView{}, nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unavailable := &apierror.ServiceUnavailableError{Msg: "snclient: connection reset"}
			mirror := &transferMirror{err: unavailable, getCase: tc.getCase}
			repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
			svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, nil, "")
			svc = WithCaseTypeTransfer(svc, repo)

			_, err := svc.UpdateCase(contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com")), engagementTransfer())
			if !errors.Is(err, unavailable) {
				t.Fatalf("want ServiceNow's own error back, got %v", err)
			}
			if repo.remoteOK || len(repo.activity) != 0 {
				t.Errorf("a transfer ServiceNow did not apply must leave nothing behind: remoteOK=%v activity=%v", repo.remoteOK, repo.activity)
			}
		})
	}
}

// A refusal is definite: ServiceNow looked at the request and said no, so there
// is nothing to re-check, and asking again would only be a wasted round trip.
func TestCaseService_UpdateCase_ServiceNowRefusalIsNotRechecked(t *testing.T) {
	for name, refusal := range map[string]error{
		"400": &apierror.ValidationError{Msg: "no"}, "401": &apierror.UnauthorizedError{Msg: "no"},
		"403": &apierror.ForbiddenError{Msg: "no"}, "404": &apierror.NotFoundError{Msg: "no"}, "409": &apierror.ConflictError{Msg: "no"},
	} {
		t.Run(name, func(t *testing.T) {
			mirror := &transferMirror{err: refusal}
			repo := newTransferStubRepo(repository.CaseTypeTransferResult{PreviousType: "case"})
			svc := NewCaseServiceWithSNWriteback(repo, transferUsers(), nil, alwaysUnrestrictedAccess{}, nil, nil, mirror, nil, "")
			svc = WithCaseTypeTransfer(svc, repo)

			if _, err := svc.UpdateCase(context.Background(), engagementTransfer()); !errors.Is(err, refusal) {
				t.Fatalf("want the refusal back, got %v", err)
			}
			if mirror.getCalls != 0 {
				t.Errorf("a definite refusal was re-checked %d time(s)", mirror.getCalls)
			}
		})
	}
}

// The ServiceNow call is bounded so that, if it is slow, the request still has
// time to ask whether it applied the transfer and to commit.
func TestServiceNowTransferContext(t *testing.T) {
	t.Run("a request with plenty of time leaves a margin after the call", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		ctx, cancelCall := serviceNowTransferContext(parent)
		defer cancelCall()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("no deadline on the call")
		}
		parentDeadline, _ := parent.Deadline()
		if left := parentDeadline.Sub(deadline); left < serviceNowTransferMargin-time.Second || left > serviceNowTransferMargin+time.Second {
			t.Errorf("call ends %s before the request's deadline, want about %s", left, serviceNowTransferMargin)
		}
	})
	t.Run("a request with little time left is left alone", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ctx, cancelCall := serviceNowTransferContext(parent)
		defer cancelCall()
		if ctx != parent {
			t.Errorf("a request with only 10s left should not be given a shorter call")
		}
	})
	t.Run("a request with no deadline is left alone", func(t *testing.T) {
		parent := context.Background()
		ctx, cancelCall := serviceNowTransferContext(parent)
		defer cancelCall()
		if ctx != parent {
			t.Errorf("a context with no deadline should be returned as is")
		}
	})
}
