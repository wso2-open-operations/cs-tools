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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	testProjectSfID      = "a0dE200000RgxAbIAJ"
	testProjectKey       = "CSMSYNCTEST"
	testProjectAccountSf = "001E2000025AYH2IAO"
	testProjectAccountID = "22222222-3333-4444-5555-666666666666"
	testProjectTypeID    = "33333333-4444-5555-6666-777777777777"
	testProjectRowID     = "44444444-5555-6666-7777-888888888888"
)

type fakeProjectSalesEntity struct {
	project salesentity.Project
	err     error
	calls   int
	lastID  string
}

func (f *fakeProjectSalesEntity) GetProject(_ context.Context, id string) (salesentity.Project, error) {
	f.calls++
	f.lastID = id
	return f.project, f.err
}

// fakeProjectRepo is an in-memory project table keyed by sf_id. With
// allowInsert false in the upsert, a missing sf_id is the repository's
// NotFoundError, as writeSalesforceProject returns it.
type fakeProjectRepo struct {
	bySfID      map[string]string
	types       map[string]string
	upserts     []domain.SalesforceProjectUpsert
	upsertState []domain.UpsertSalesforceIngestStateRequest
	upsertErr   error
	deletes     []string
	deleteState []domain.UpsertSalesforceIngestStateRequest
	deleteFound bool
	lookups     []string
}

func (f *fakeProjectRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceProjectUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceProjectUpsertResult, error) {
	f.upserts = append(f.upserts, row)
	f.upsertState = append(f.upsertState, state)
	if f.upsertErr != nil {
		return domain.SalesforceProjectUpsertResult{}, f.upsertErr
	}
	if id, ok := f.bySfID[row.SfID]; ok {
		return domain.SalesforceProjectUpsertResult{ProjectID: id}, nil
	}
	if !row.AllowInsert {
		return domain.SalesforceProjectUpsertResult{}, &apierror.NotFoundError{Msg: fmt.Sprintf("project not found for sfId %q / key %q", row.SfID, row.Key)}
	}
	if f.bySfID == nil {
		f.bySfID = map[string]string{}
	}
	f.bySfID[row.SfID] = testProjectRowID
	return domain.SalesforceProjectUpsertResult{ProjectID: testProjectRowID, Created: true}, nil
}

func (f *fakeProjectRepo) SoftDeleteBySfID(_ context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	f.deletes = append(f.deletes, sfID)
	f.deleteState = append(f.deleteState, state)
	return f.deleteFound, nil
}

func (f *fakeProjectRepo) LookupProjectIDBySfID(_ context.Context, sfID string) (*string, error) {
	f.lookups = append(f.lookups, sfID)
	if id, ok := f.bySfID[sfID]; ok {
		return &id, nil
	}
	return nil, nil
}

func (f *fakeProjectRepo) LookupProjectTypeIDByName(_ context.Context, name string) (*string, error) {
	if id, ok := f.types[name]; ok {
		return &id, nil
	}
	return nil, nil
}

func sampleProject() salesentity.Project {
	return salesentity.Project{
		ID:                      testProjectSfID,
		Name:                    sampleStr("CSM Sync Test"),
		Key:                     sampleStr(testProjectKey),
		Description:             sampleStr("Sync test project"),
		Type:                    sampleStr("Subscription"),
		StartDate:               sampleStr("2026-04-01"),
		EndDate:                 sampleStr("2027-03-31"),
		ComplianceViolationDate: sampleStr("2027-01-15"),
		GoLiveDate:              sampleStr("2026-05-01"),
		CustomerID:              sampleStr(testProjectAccountSf),
		LastModifiedDate:        sampleStr("2026-09-29T09:28:21.000+0000"),
	}
}

func newProjectRepo(existing map[string]string) *fakeProjectRepo {
	if existing == nil {
		existing = map[string]string{}
	}
	return &fakeProjectRepo{bySfID: existing, types: map[string]string{"Subscription": testProjectTypeID}, deleteFound: true}
}

// newProjectService builds a service with the Project ingest on, the Account
// ingest off (EnsureAccount is a pure lookup) and the test account in CSM.
func newProjectService(se *fakeProjectSalesEntity, repo *fakeProjectRepo, states *fakeIngestStateRepo, insert bool) *salesforceEventService {
	lookup := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{testProjectAccountSf: testProjectAccountID}}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: states, Projects: repo})
	return WithProjectIngest(svc, ProjectIngest{Projects: repo, SalesEntity: se, InsertEnabled: insert}).(*salesforceEventService)
}

func projectEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{Entity: "Project__c", EventType: eventType, ReferenceID: testProjectSfID}
}

func TestProjectIngest_DisabledIgnoresEvents(t *testing.T) {
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}})
	for _, et := range []string{"CREATED", "UPDATED", "DELETED", "RESTORED"} {
		if err := svc.HandleEvent(context.Background(), projectEvent(et)); err != nil {
			t.Errorf("%s: err = %v, want nil (acknowledged)", et, err)
		}
	}
	var re ProjectReingester = svc.(*salesforceEventService)
	if err := re.RetryProjectIngest(context.Background(), testProjectSfID); !errors.Is(err, errProjectIngestDisabled) {
		t.Errorf("retry err = %v, want errProjectIngestDisabled", err)
	}
}

// TestProjectIngest_UpdatesExistingProject: CREATED, UPDATED and RESTORED
// fetch the project and write the mapped Salesforce-owned columns with a
// SUCCEEDED ledger row; only RESTORED asks for reactivation.
func TestProjectIngest_UpdatesExistingProject(t *testing.T) {
	for _, et := range []string{"CREATED", "UPDATED", "RESTORED"} {
		t.Run(et, func(t *testing.T) {
			se := &fakeProjectSalesEntity{project: sampleProject()}
			repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
			states := &fakeIngestStateRepo{}
			svc := newProjectService(se, repo, states, false)
			if err := svc.HandleEvent(context.Background(), projectEvent(et)); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if len(repo.upserts) != 1 {
				t.Fatalf("upserts = %d", len(repo.upserts))
			}
			row := repo.upserts[0]
			date := func(p *time.Time) string {
				if p == nil {
					return ""
				}
				return p.Format(time.DateOnly)
			}
			if row.SfID != testProjectSfID || row.Key != testProjectKey || derefString(row.Name) != "CSM Sync Test" ||
				derefString(row.AccountID) != testProjectAccountID || derefString(row.ProjectTypeID) != testProjectTypeID ||
				derefString(row.Description) != "Sync test project" || date(row.StartDate) != "2026-04-01" || date(row.EndDate) != "2027-03-31" ||
				date(row.ComplianceViolationDate) != "2027-01-15" || date(row.GoLiveDate) != "2026-05-01" || row.AllowInsert {
				t.Errorf("row = %+v", row)
			}
			if row.Reactivate != (et == "RESTORED") {
				t.Errorf("reactivate = %v for %s", row.Reactivate, et)
			}
			st := repo.upsertState[0]
			if st.Entity != domain.SalesforceIngestEntityProject || st.EventType != et || st.Status != domain.SalesforceIngestSucceeded ||
				!st.EventModifiedOn.Equal(time.Date(2026, 9, 29, 9, 28, 21, 0, time.UTC)) {
				t.Errorf("ledger state = %+v", st)
			}
			if len(states.upserts) != 0 {
				t.Errorf("out-of-transaction ledger writes = %+v, want none on success", states.upserts)
			}
		})
	}
}

// TestProjectIngest_UpdateOnlyMissingProjectIsNotFound: with inserts off, a
// project CSM does not have fails with a NotFoundError and a FAILED ledger
// row the retry job recognises.
func TestProjectIngest_UpdateOnlyMissingProjectIsNotFound(t *testing.T) {
	repo := newProjectRepo(nil)
	states := &fakeIngestStateRepo{}
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, states, false)
	err := svc.HandleEvent(context.Background(), projectEvent("UPDATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "project not found for sfId") {
		t.Fatalf("err = %v, want NotFoundError \"project not found for sfId ...\"", err)
	}
	if len(states.upserts) != 1 {
		t.Fatalf("ledger writes = %+v", states.upserts)
	}
	st := states.upserts[0]
	if st.Status != domain.SalesforceIngestFailed || st.Entity != domain.SalesforceIngestEntityProject || !repository.IsMissingParentError(derefString(st.LastError)) {
		t.Errorf("ledger row = %+v (%q)", st, derefString(st.LastError))
	}
}

func TestProjectIngest_InsertSwitchCreates(t *testing.T) {
	repo := newProjectRepo(nil)
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true)
	if err := svc.HandleEvent(context.Background(), projectEvent("CREATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(repo.upserts) != 1 || !repo.upserts[0].AllowInsert || repo.bySfID[testProjectSfID] != testProjectRowID {
		t.Errorf("upserts = %+v, rows = %v", repo.upserts, repo.bySfID)
	}
}

// TestProjectIngest_EmptyKeyRefused (D5): a project without Project_Key__c
// is recorded FAILED with the reason and acknowledged, never written.
func TestProjectIngest_EmptyKeyRefused(t *testing.T) {
	for _, key := range []*string{nil, sampleStr(""), sampleStr("   ")} {
		p := sampleProject()
		p.Key = key
		repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
		states := &fakeIngestStateRepo{}
		svc := newProjectService(&fakeProjectSalesEntity{project: p}, repo, states, true)
		if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); err != nil {
			t.Fatalf("key %q: err = %v, want nil (acknowledged)", derefString(key), err)
		}
		if len(repo.upserts) != 0 {
			t.Errorf("key %q: upserts = %d, want 0", derefString(key), len(repo.upserts))
		}
		if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed ||
			!strings.Contains(derefString(states.upserts[0].LastError), "Project_Key__c") {
			t.Errorf("key %q: ledger = %+v", derefString(key), states.upserts)
		}
	}
	// The retry job's re-run acknowledges it too.
	p := sampleProject()
	p.Key = nil
	svc := newProjectService(&fakeProjectSalesEntity{project: p}, newProjectRepo(nil), &fakeIngestStateRepo{}, true)
	if err := svc.RetryProjectIngest(context.Background(), testProjectSfID); err != nil {
		t.Errorf("retry err = %v, want nil", err)
	}
}

// TestProjectIngest_UnknownProjectTypeWritesNull: a label with no
// project_type row (e.g. "Private Cloud Subscription") writes NULL and never
// creates a type.
func TestProjectIngest_UnknownProjectTypeWritesNull(t *testing.T) {
	for _, typ := range []*string{sampleStr("Private Cloud Subscription"), nil} {
		p := sampleProject()
		p.Type = typ
		repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
		svc := newProjectService(&fakeProjectSalesEntity{project: p}, repo, &fakeIngestStateRepo{}, false)
		if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); err != nil {
			t.Fatalf("HandleEvent: %v", err)
		}
		if repo.upserts[0].ProjectTypeID != nil {
			t.Errorf("type %q: project_type_id = %v, want NULL", derefString(typ), *repo.upserts[0].ProjectTypeID)
		}
	}
}

func TestProjectIngest_AccountNotFoundRecordsFailed(t *testing.T) {
	p := sampleProject()
	p.CustomerID = sampleStr("001E200000ZZZZZIAS")
	repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
	states := &fakeIngestStateRepo{}
	svc := newProjectService(&fakeProjectSalesEntity{project: p}, repo, states, false)
	var nf *apierror.NotFoundError
	if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
	if len(repo.upserts) != 0 || len(states.upserts) != 1 || !repository.IsMissingParentError(derefString(states.upserts[0].LastError)) {
		t.Errorf("upserts = %d ledger = %+v", len(repo.upserts), states.upserts)
	}
}

func TestProjectIngest_NoCustomerKeepsAccount(t *testing.T) {
	p := sampleProject()
	p.CustomerID = nil
	repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
	svc := newProjectService(&fakeProjectSalesEntity{project: p}, repo, &fakeIngestStateRepo{}, false)
	if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.upserts[0].AccountID != nil {
		t.Errorf("account id = %v, want nil (keep stored)", *repo.upserts[0].AccountID)
	}
}

func TestProjectIngest_GuardSkipsSameVersion(t *testing.T) {
	key := domain.SalesforceIngestEntityProject + "/" + testProjectSfID
	states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{key: {
		Entity: domain.SalesforceIngestEntityProject, SfID: testProjectSfID, Status: domain.SalesforceIngestSucceeded,
		EventType: "UPDATED", EventModifiedOn: time.Date(2026, 9, 29, 9, 28, 21, 0, time.UTC),
	}}}
	repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, states, false)
	if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(repo.upserts) != 0 {
		t.Errorf("upserts = %d, want 0 (duplicate)", len(repo.upserts))
	}
}

func TestProjectIngest_UpsertErrorRecordsFailed(t *testing.T) {
	repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
	repo.upsertErr = errors.New("boom")
	states := &fakeIngestStateRepo{}
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, states, false)
	if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed {
		t.Errorf("ledger = %+v", states.upserts)
	}
}

func TestProjectIngest_FetchErrorPropagates(t *testing.T) {
	se := &fakeProjectSalesEntity{err: &apierror.ServiceUnavailableError{Msg: "salesentity: project not in search results"}}
	repo := newProjectRepo(nil)
	states := &fakeIngestStateRepo{}
	svc := newProjectService(se, repo, states, false)
	var sue *apierror.ServiceUnavailableError
	if err := svc.HandleEvent(context.Background(), projectEvent("UPDATED")); !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if len(repo.upserts) != 0 || len(states.upserts) != 0 {
		t.Errorf("writes after failed fetch: %d %d", len(repo.upserts), len(states.upserts))
	}
}

// TestProjectIngest_DeletedSoftMarks: DELETED marks the project with a
// DELETED ledger row, widening a 15-character id, once a lookup confirms it is gone; a project
// CSM does not have is acknowledged.
func TestProjectIngest_DeletedSoftMarks(t *testing.T) {
	for _, found := range []bool{true, false} {
		se := &fakeProjectSalesEntity{err: salesentity.NotFound("salesentity: project not in search results")}
		repo := newProjectRepo(nil)
		repo.deleteFound = found
		svc := newProjectService(se, repo, &fakeIngestStateRepo{}, false)
		req := projectEvent("DELETED")
		req.ReferenceID = testProjectSfID[:15]
		if err := svc.HandleEvent(context.Background(), req); err != nil {
			t.Fatalf("found=%v: err = %v", found, err)
		}
		if se.calls != 1 || len(repo.deletes) != 1 || repo.deletes[0] != testProjectSfID {
			t.Errorf("found=%v: fetches %d deletes %v", found, se.calls, repo.deletes)
		}
		st := repo.deleteState[0]
		if st.Entity != domain.SalesforceIngestEntityProject || st.EventType != domain.SalesforceEventDeleted || st.Status != domain.SalesforceIngestSucceeded || st.SfID != testProjectSfID {
			t.Errorf("found=%v: state = %+v", found, st)
		}
	}
}

// TestEnsureProject covers the three outcomes: present (one lookup),
// missing with inserts off (NotFoundError for the retry job), missing with
// inserts on (fetched, inserted, read back).
func TestEnsureProject(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		se := &fakeProjectSalesEntity{project: sampleProject()}
		repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
		svc := newProjectService(se, repo, &fakeIngestStateRepo{}, true)
		id, err := svc.EnsureProject(context.Background(), testProjectSfID[:15])
		if err != nil || id != testProjectRowID || se.calls != 0 {
			t.Errorf("id=%q err=%v fetches=%d", id, err, se.calls)
		}
	})
	t.Run("missing, inserts off", func(t *testing.T) {
		se := &fakeProjectSalesEntity{project: sampleProject()}
		svc := newProjectService(se, newProjectRepo(nil), &fakeIngestStateRepo{}, false)
		_, err := svc.EnsureProject(context.Background(), testProjectSfID)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) || !repository.IsMissingParentError(nf.Msg) || se.calls != 0 {
			t.Errorf("err=%v fetches=%d", err, se.calls)
		}
	})
	t.Run("missing, project ingest off", func(t *testing.T) {
		repo := newProjectRepo(nil)
		svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}, Projects: repo}).(*salesforceEventService)
		_, err := svc.EnsureProject(context.Background(), testProjectSfID)
		var nf *apierror.NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("err=%v, want NotFoundError", err)
		}
	})
	t.Run("missing, inserts on", func(t *testing.T) {
		se := &fakeProjectSalesEntity{project: sampleProject()}
		repo := newProjectRepo(nil)
		svc := newProjectService(se, repo, &fakeIngestStateRepo{}, true)
		id, err := svc.EnsureProject(context.Background(), testProjectSfID)
		if err != nil || id != testProjectRowID || se.calls != 1 || len(repo.upserts) != 1 {
			t.Errorf("id=%q err=%v fetches=%d upserts=%d", id, err, se.calls, len(repo.upserts))
		}
	})
	t.Run("missing, keyless", func(t *testing.T) {
		p := sampleProject()
		p.Key = nil
		svc := newProjectService(&fakeProjectSalesEntity{project: p}, newProjectRepo(nil), &fakeIngestStateRepo{}, true)
		var ve *apierror.ValidationError
		if _, err := svc.EnsureProject(context.Background(), testProjectSfID); !errors.As(err, &ve) || !errors.Is(err, errProjectKeyMissing) {
			t.Errorf("err=%v, want a ValidationError matching errProjectKeyMissing", err)
		}
	})
}

// sequencedMembershipRepo answers the membership upserts with errs in turn
// (nil once they run out).
type sequencedMembershipRepo struct {
	*fakeMembershipRepo
	errs []error
}

func (r *sequencedMembershipRepo) Upsert(ctx context.Context, in domain.SalesforceMembershipUpsert, step domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error) {
	r.fakeMembershipRepo.upsertErr = nil
	if len(r.errs) > 0 {
		r.fakeMembershipRepo.upsertErr, r.errs = r.errs[0], r.errs[1:]
	}
	return r.fakeMembershipRepo.Upsert(ctx, in, step)
}

// TestMembershipIngest_EnsuresMissingProject: a membership whose project is
// not in CSM ingests the project and retries once when project inserts are
// allowed; otherwise it keeps today's NotFoundError for the retry job.
func TestMembershipIngest_EnsuresMissingProject(t *testing.T) {
	notFound := &apierror.NotFoundError{Msg: fmt.Sprintf("project not found for key %q / sfId %q", "ACMEPROD", testProjectID)}
	for _, tc := range []struct {
		name        string
		insert      bool
		wantErr     bool
		wantUpserts int
	}{
		{"inserts on", true, false, 2},
		{"inserts off", false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newIngestHarness(sampleProjectContact(domain.MembershipStateRegistered, "Portal user"), sampleContact(), false)
			repo := &sequencedMembershipRepo{fakeMembershipRepo: h.repo, errs: []error{notFound}}
			p := sampleProject()
			p.ID = testProjectID
			se := &fakeProjectSalesEntity{project: p}
			projects := newProjectRepo(nil)
			svc := NewSalesforceEventServiceWithMembershipIngest(h.accounts, &stubSalesEntityClient{},
				SalesforceIngestSupport{Accounts: &stubSalesforceAccountRepo{accountsBySfID: map[string]string{testProjectAccountSf: testProjectAccountID}}, States: h.states, Projects: projects},
				MembershipIngest{Memberships: repo, Steps: h.steps, SalesEntity: h.se, Contacts: h.contacts})
			svc = WithProjectIngest(svc, ProjectIngest{Projects: projects, SalesEntity: se, InsertEnabled: tc.insert})
			err := svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c"))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(h.repo.upserts) != tc.wantUpserts {
				t.Errorf("membership upserts = %d, want %d", len(h.repo.upserts), tc.wantUpserts)
			}
			if tc.insert && (se.lastID != testProjectID || projects.bySfID[testProjectID] == "") {
				t.Errorf("project fetch %q rows %v, want the membership's project ingested", se.lastID, projects.bySfID)
			}
			if !tc.insert && se.calls != 0 {
				t.Errorf("project fetches = %d, want 0 with inserts off", se.calls)
			}
		})
	}
}

// TestRetryWorker_ProjectRetrier: a FAILED "project not found" project row
// is handed to the registered retrier.
func TestRetryWorker_ProjectRetrier(t *testing.T) {
	msg := fmt.Sprintf("project not found for sfId %q / key %q", testProjectSfID, testProjectKey)
	states := &fakeIngestStateRepo{failed: []domain.SalesforceIngestState{{
		Entity: domain.SalesforceIngestEntityProject, SfID: testProjectSfID, Status: domain.SalesforceIngestFailed, LastError: &msg, AttemptCount: 1,
	}}}
	repo := newProjectRepo(map[string]string{testProjectSfID: testProjectRowID})
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, false)
	w := NewSalesforceIngestRetryWorker(nil, nil, states, time.Minute)
	w.EntityRetriers[domain.SalesforceIngestEntityProject] = svc.RetryProjectIngest
	w.RunOnce(context.Background())
	if len(repo.upserts) != 1 {
		t.Errorf("upserts = %d, want the retrier to re-run the project", len(repo.upserts))
	}
}
