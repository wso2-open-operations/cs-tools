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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// stubProjectUpdateRepo is a minimal repository.ProjectRepository whose
// UpdateProject records the call and returns a canned result -- used to
// exercise pgProjectUpdateService without a real Postgres connection.
type stubProjectUpdateRepo struct {
	updateProject func(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error)

	called    bool
	gotID     string
	gotReq    domain.ProjectUpdateRequest
	gotByWhom string
}

func (r *stubProjectUpdateRepo) SearchProjects(context.Context, domain.SearchProjectsRequest, repository.SearchScope) ([]domain.Project, int, error) {
	panic("SearchProjects: not exercised by these tests")
}

func (r *stubProjectUpdateRepo) GetProjectByID(context.Context, string, repository.SearchScope) (domain.ProjectDetailsView, error) {
	panic("GetProjectByID: not exercised by these tests")
}

func (r *stubProjectUpdateRepo) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest, updatedBy string) (domain.ProjectUpdateResult, error) {
	r.called = true
	r.gotID = id
	r.gotReq = req
	r.gotByWhom = updatedBy
	if r.updateProject != nil {
		return r.updateProject(ctx, id, req, updatedBy)
	}
	return domain.ProjectUpdateResult{ID: id, UpdatedBy: updatedBy, UpdatedOn: time.Now()}, nil
}

// stubProjectMirror is a ProjectUpdateService whose UpdateProject records the
// call and returns either a canned error or a canned response -- used as the
// dual-write mode's ServiceNow mirror, standing in for the real
// snProjectUpdateService so these tests never make a live ServiceNow call.
type stubProjectMirror struct {
	err error

	calls  int
	gotID  string
	gotReq domain.ProjectUpdateRequest
}

func (m *stubProjectMirror) UpdateProject(_ context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	m.calls++
	m.gotID = id
	m.gotReq = req
	if m.err != nil {
		return domain.ProjectUpdateResponse{}, m.err
	}
	return domain.ProjectUpdateResponse{Message: "mirrored"}, nil
}

func testProjectUserRepo() stubUserRepo {
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			return domain.User{ID: "u-1", Email: email}, nil
		},
	}
}

// TestPgProjectUpdateService_RejectsSuspensionProcessState locks in that
// suspensionProcessState is rejected with a ValidationError rather than
// silently dropped -- it has no Postgres column anywhere for project (see
// pgProjectUpdateService's own doc comment) -- and that the repository is
// never called in that case.
func TestPgProjectUpdateService_RejectsSuspensionProcessState(t *testing.T) {
	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo())
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{
		SuspensionProcessState: []byte(`{"a":1}`),
	})

	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("UpdateProject() error = %v, want *apierror.ValidationError", err)
	}
	if repo.called {
		t.Fatal("UpdateProject() called the repository despite an unsupported field")
	}
}

// TestPgProjectUpdateService_RequiresAtLeastOneField mirrors the
// ServiceNow-mode contract's own "at least one field must be provided" rule.
func TestPgProjectUpdateService_RequiresAtLeastOneField(t *testing.T) {
	repo := &stubProjectUpdateRepo{}
	svc := NewProjectUpdateService(repo, testProjectUserRepo())
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateProject(ctx, "11111111-1111-1111-1111-111111111111", domain.ProjectUpdateRequest{})

	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("UpdateProject() error = %v, want *apierror.ValidationError", err)
	}
}

// TestPgProjectUpdateService_PlainPostgresUpdatesFieldsAndNeverCallsSN proves
// a plain DATA_SOURCE=postgres pgProjectUpdateService (constructed via
// NewProjectUpdateService, with no dispatcher/mirror at all) applies the
// requested fields through the repository and returns the repository's
// result untouched -- and, since there is no snWriteback/snMirror wiring in
// this mode, there is no way for it to reach ServiceNow.
func TestPgProjectUpdateService_PlainPostgresUpdatesFieldsAndNeverCallsSN(t *testing.T) {
	endDateState := "NOTIFIED"
	hasAgent := true
	wantResult := domain.ProjectUpdateResult{
		ID:                  "11111111-1111-1111-1111-111111111111",
		UpdatedBy:           "jane.doe@example.com",
		UpdatedOn:           time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
		EndDateClosureState: &endDateState,
	}
	repo := &stubProjectUpdateRepo{
		updateProject: func(_ context.Context, _ string, _ domain.ProjectUpdateRequest, _ string) (domain.ProjectUpdateResult, error) {
			return wantResult, nil
		},
	}
	svc := NewProjectUpdateService(repo, testProjectUserRepo())
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.UpdateProject(ctx, wantResult.ID, domain.ProjectUpdateRequest{
		HasAgent:            &hasAgent,
		EndDateClosureState: &endDateState,
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if !repo.called {
		t.Fatal("UpdateProject() never called the repository")
	}
	if repo.gotByWhom != "jane.doe@example.com" {
		t.Fatalf("repo.UpdateProject() updatedBy = %q, want the caller's own resolved email", repo.gotByWhom)
	}
	if repo.gotReq.HasAgent == nil || !*repo.gotReq.HasAgent {
		t.Fatalf("repo.UpdateProject() req.HasAgent = %v, want true", repo.gotReq.HasAgent)
	}
	if resp.Project.ID != wantResult.ID || resp.Project.UpdatedBy != wantResult.UpdatedBy ||
		!resp.Project.UpdatedOn.Equal(wantResult.UpdatedOn) ||
		resp.Project.EndDateClosureState == nil || *resp.Project.EndDateClosureState != *wantResult.EndDateClosureState {
		t.Fatalf("UpdateProject() response.Project = %+v, want the repository's own result untouched: %+v", resp.Project, wantResult)
	}
}

// TestPgProjectUpdateService_DualWriteDispatchesExactlyOneMirrorCallOnSuccess
// proves DATA_SOURCE=postgres-servicenow-dual-write mode dispatches exactly
// one best-effort ServiceNow mirror write after a successful Postgres write,
// and that a mirror failure neither fails the caller's response nor retries
// -- it lands in sn_writeback_failures via SNWritebackDispatcher.run, which
// this test doesn't need to reach directly since SNWritebackDispatcher's own
// test already covers that recording behavior; this test only needs to prove
// pgProjectUpdateService dispatches the mirror call and tolerates it failing.
func TestPgProjectUpdateService_DualWriteDispatchesExactlyOneMirrorCallOnSuccess(t *testing.T) {
	endDateState := "NOTIFIED"
	repo := &stubProjectUpdateRepo{}
	mirror := &stubProjectMirror{err: errors.New("sn unreachable")}
	failures := &recordingSNWritebackFailures{}
	dispatcher := NewSNWritebackDispatcher(failures)
	svc := NewProjectUpdateServiceWithSNWriteback(repo, testProjectUserRepo(), dispatcher, mirror)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	id := "11111111-1111-1111-1111-111111111111"
	resp, err := svc.UpdateProject(ctx, id, domain.ProjectUpdateRequest{EndDateClosureState: &endDateState})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v -- a failed/slow ServiceNow mirror must never fail the Postgres-backed response", err)
	}
	if resp.Message == "" {
		t.Fatal("UpdateProject() returned an empty response despite a successful Postgres write")
	}

	// The mirror is configured to fail (mirror.err set above) -- proves a
	// mirror failure both doesn't retry (exactly one call) and lands in
	// sn_writeback_failures via SNWritebackDispatcher.run.
	waitFor(t, func() bool { return failures.count() == 1 })
	if mirror.calls != 1 {
		t.Fatalf("ServiceNow mirror UpdateProject called %d times, want exactly 1", mirror.calls)
	}
	if mirror.gotID != id {
		t.Fatalf("ServiceNow mirror UpdateProject id = %q, want %q", mirror.gotID, id)
	}
	if mirror.gotReq.SuspensionProcessState != nil {
		t.Fatal("ServiceNow mirror UpdateProject req carried suspensionProcessState, which this data source never persisted")
	}
}
