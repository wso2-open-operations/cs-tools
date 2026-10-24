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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// testBackingProjectID is the id the fake backing system uses for the test project.
const testBackingProjectID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

type fakeProjectIDResolver struct {
	id    string
	found bool
	err   error
	keys  []string
}

func (f *fakeProjectIDResolver) ResolveProjectID(_ context.Context, key string) (string, bool, error) {
	f.keys = append(f.keys, key)
	return f.id, f.found, f.err
}

func newResolvingProjectService(se *fakeProjectSalesEntity, repo *fakeProjectRepo, states *fakeIngestStateRepo, insert bool, resolver ProjectIDResolver) *salesforceEventService {
	svc := newProjectService(se, repo, states, insert)
	svc.project.IDResolver = resolver
	return svc
}

// TestProjectIngest_DualWriteInsertsWithBackingID: a project to insert takes
// the id the backing system returns for its key.
func TestProjectIngest_DualWriteInsertsWithBackingID(t *testing.T) {
	repo := newProjectRepo(nil)
	res := &fakeProjectIDResolver{id: testBackingProjectID, found: true}
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true, res)
	if err := svc.HandleEvent(context.Background(), projectEvent("CREATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(res.keys) != 1 || res.keys[0] != testProjectKey {
		t.Errorf("resolver keys = %v, want [%s]", res.keys, testProjectKey)
	}
	if len(repo.upserts) != 1 {
		t.Fatalf("upserts = %d", len(repo.upserts))
	}
	row := repo.upserts[0]
	if row.InsertID == nil || *row.InsertID != testBackingProjectID || !row.RequireInsertID {
		t.Errorf("upsert InsertID = %v RequireInsertID = %v, want the backing id and true", row.InsertID, row.RequireInsertID)
	}
	if repo.bySfID[testProjectSfID] != testBackingProjectID {
		t.Errorf("row id = %q, want %q", repo.bySfID[testProjectSfID], testBackingProjectID)
	}
}

// TestProjectIngest_DualWriteMissIsRetryableNotFound: the backing system does
// not have the project yet. Nothing is written, no random id is used, and the
// FAILED ledger row is one the retry job re-runs.
func TestProjectIngest_DualWriteMissIsRetryableNotFound(t *testing.T) {
	repo := newProjectRepo(nil)
	states := &fakeIngestStateRepo{}
	res := &fakeProjectIDResolver{found: false}
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, states, true, res)
	err := svc.HandleEvent(context.Background(), projectEvent("CREATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "project not found") {
		t.Fatalf("err = %v, want NotFoundError starting \"project not found\"", err)
	}
	if len(repo.upserts) != 0 || len(repo.bySfID) != 0 {
		t.Errorf("upserts = %d rows = %v, want no write on a miss", len(repo.upserts), repo.bySfID)
	}
	if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed ||
		!repository.IsMissingParentError(derefString(states.upserts[0].LastError)) {
		t.Errorf("ledger = %+v, want one FAILED missing-parent row", states.upserts)
	}
}

// TestProjectIngest_DualWriteRetryInsertsOnceBackingHasIt: the retry job's
// re-run succeeds when the backing system has caught up.
func TestProjectIngest_DualWriteRetryInsertsOnceBackingHasIt(t *testing.T) {
	repo := newProjectRepo(nil)
	res := &fakeProjectIDResolver{found: false}
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true, res)
	if err := svc.RetryProjectIngest(context.Background(), testProjectSfID); err == nil {
		t.Fatal("first run: want the not-found error")
	}
	res.id, res.found = testBackingProjectID, true
	if err := svc.RetryProjectIngest(context.Background(), testProjectSfID); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if repo.bySfID[testProjectSfID] != testBackingProjectID {
		t.Errorf("row id = %q, want %q", repo.bySfID[testProjectSfID], testBackingProjectID)
	}
}

func TestProjectIngest_DualWriteResolverErrorWritesNothing(t *testing.T) {
	repo := newProjectRepo(nil)
	states := &fakeIngestStateRepo{}
	boom := &apierror.ConflictError{Msg: "two projects"}
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, states, true, &fakeProjectIDResolver{err: boom})
	err := svc.HandleEvent(context.Background(), projectEvent("CREATED"))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the resolver error", err)
	}
	if len(repo.upserts) != 0 {
		t.Errorf("upserts = %d, want none", len(repo.upserts))
	}
	if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed || repository.IsMissingParentError(derefString(states.upserts[0].LastError)) {
		t.Errorf("ledger = %+v, want one FAILED row the retry job does not match", states.upserts)
	}
}

// TestProjectIngest_PostgresModeNoLookup: with no resolver (plain postgres)
// nothing is looked up and the insert leaves the id to the database.
func TestProjectIngest_PostgresModeNoLookup(t *testing.T) {
	repo := newProjectRepo(nil)
	svc := newProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true)
	if err := svc.HandleEvent(context.Background(), projectEvent("CREATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	row := repo.upserts[0]
	if row.InsertID != nil || row.RequireInsertID || repo.existsCalls != 0 {
		t.Errorf("InsertID = %v RequireInsertID = %v existsCalls = %d, want nil, false, 0", row.InsertID, row.RequireInsertID, repo.existsCalls)
	}
	if repo.bySfID[testProjectSfID] != testProjectRowID {
		t.Errorf("row id = %q, want the database-generated one", repo.bySfID[testProjectSfID])
	}
}

// TestProjectIngest_DualWriteExistingRowNoLookup: a project already in CSM,
// found by sf_id (update, replay, duplicate) or by key, costs no backing call
// and carries no InsertID.
func TestProjectIngest_DualWriteExistingRowNoLookup(t *testing.T) {
	cases := map[string]*fakeProjectRepo{
		"by sf_id": newProjectRepo(map[string]string{testProjectSfID: testProjectRowID}),
		"by key": func() *fakeProjectRepo {
			r := newProjectRepo(nil)
			r.byKey = map[string]string{testProjectKey: testProjectRowID}
			return r
		}(),
	}
	for name, repo := range cases {
		for _, et := range []string{"CREATED", "UPDATED", "RESTORED"} {
			res := &fakeProjectIDResolver{id: testBackingProjectID, found: true}
			svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true, res)
			if err := svc.HandleEvent(context.Background(), projectEvent(et)); err != nil {
				t.Fatalf("%s %s: %v", name, et, err)
			}
			if len(res.keys) != 0 {
				t.Errorf("%s %s: resolver called %d times, want 0", name, et, len(res.keys))
			}
		}
		for i, u := range repo.upserts {
			if u.InsertID != nil {
				t.Errorf("%s upsert %d: InsertID = %v, want nil", name, i, u.InsertID)
			}
		}
	}
}

// TestProjectIngest_DualWriteUpdateOnlyNoLookup: with the insert switch off
// the ingest never creates a project, so it never asks the backing system.
func TestProjectIngest_DualWriteUpdateOnlyNoLookup(t *testing.T) {
	repo := newProjectRepo(nil)
	res := &fakeProjectIDResolver{id: testBackingProjectID, found: true}
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, false, res)
	err := svc.HandleEvent(context.Background(), projectEvent("UPDATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || len(res.keys) != 0 || repo.existsCalls != 0 {
		t.Errorf("err = %v, resolver calls = %d, exists calls = %d", err, len(res.keys), repo.existsCalls)
	}
}

// TestEnsureProject_DualWriteMissIsRetryable: a child ingest that needs the
// parent first gets the same retryable error, not a random-id project.
func TestEnsureProject_DualWriteMissIsRetryable(t *testing.T) {
	repo := newProjectRepo(nil)
	svc := newResolvingProjectService(&fakeProjectSalesEntity{project: sampleProject()}, repo, &fakeIngestStateRepo{}, true, &fakeProjectIDResolver{})
	_, err := svc.EnsureProject(context.Background(), testProjectSfID)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "project not found") || len(repo.bySfID) != 0 {
		t.Fatalf("err = %v rows = %v, want a \"project not found\" NotFoundError and no row", err, repo.bySfID)
	}
}
