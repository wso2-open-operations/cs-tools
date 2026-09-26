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
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubDeploymentRepo lets each test override only the repository methods it
// exercises; any other method panics if called, which doubles as an
// assertion (e.g. "Postgres must stay untouched on SN failure").
type stubDeploymentRepo struct {
	searchDeployments              func(context.Context, domain.SearchDeploymentsRequest) ([]domain.DeploymentView, int, error)
	createDeploymentFromServiceNow func(context.Context, domain.CreateDeploymentRequest, string, string, string, time.Time) (domain.CreatedDeployment, error)
	updateDeploymentFields         func(context.Context, domain.UpdateDeploymentRequest, string) (domain.UpdatedDeployment, error)
}

func (r *stubDeploymentRepo) SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) ([]domain.DeploymentView, int, error) {
	if r.searchDeployments == nil {
		panic("stubDeploymentRepo: SearchDeployments not set")
	}
	return r.searchDeployments(ctx, req)
}

func (r *stubDeploymentRepo) CreateDeploymentFromServiceNow(ctx context.Context, req domain.CreateDeploymentRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployment, error) {
	if r.createDeploymentFromServiceNow == nil {
		panic("stubDeploymentRepo: CreateDeploymentFromServiceNow not set")
	}
	return r.createDeploymentFromServiceNow(ctx, req, id, number, createdBy, createdOn)
}

func (r *stubDeploymentRepo) UpdateDeploymentFields(ctx context.Context, req domain.UpdateDeploymentRequest, updatedBy string) (domain.UpdatedDeployment, error) {
	if r.updateDeploymentFields == nil {
		panic("stubDeploymentRepo: UpdateDeploymentFields not set")
	}
	return r.updateDeploymentFields(ctx, req, updatedBy)
}

// stubMirrorDeploymentService implements both the full DeploymentService
// interface (so it satisfies deploymentService.snMirror's field type) and
// the narrow deploymentSNCreator interface createDeploymentSNFirst actually
// type-asserts against -- mirroring stubMirrorCaseService's shape in
// case_service_test.go.
type stubMirrorDeploymentService struct {
	createDeploymentSNFirstDetailsFn func(context.Context, domain.CreateDeploymentRequest) (id, number, createdBy string, createdOn time.Time, err error)
	updateDeployment                 func(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error)
}

func (m *stubMirrorDeploymentService) SearchDeployments(context.Context, domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	panic("stubMirrorDeploymentService: SearchDeployments not exercised by these tests")
}

func (m *stubMirrorDeploymentService) CreateDeployment(context.Context, domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	panic("stubMirrorDeploymentService: CreateDeployment not exercised by these tests -- createDeploymentSNFirst calls createDeploymentSNFirstDetails instead")
}

func (m *stubMirrorDeploymentService) UpdateDeployment(ctx context.Context, req domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	if m.updateDeployment == nil {
		panic("stubMirrorDeploymentService: UpdateDeployment not set")
	}
	return m.updateDeployment(ctx, req)
}

func (m *stubMirrorDeploymentService) createDeploymentSNFirstDetails(ctx context.Context, req domain.CreateDeploymentRequest) (id, number, createdBy string, createdOn time.Time, err error) {
	if m.createDeploymentSNFirstDetailsFn == nil {
		panic("stubMirrorDeploymentService: createDeploymentSNFirstDetailsFn not set")
	}
	return m.createDeploymentSNFirstDetailsFn(ctx, req)
}

func validCreateDeploymentRequest() domain.CreateDeploymentRequest {
	devType := domain.DeploymentType("DEVELOPMENT")
	return domain.CreateDeploymentRequest{
		ProjectID:   testDeploymentUUID,
		Name:        "Test Deployment",
		Type:        &devType,
		Description: "created by a test",
	}
}

// A plain Postgres data source (no SN mirror wired at all) must keep
// rejecting CreateDeployment/UpdateDeployment exactly as before this
// dual-write path existed -- no regression for DATA_SOURCE=postgres.
func TestDeploymentService_NotSupportedOnPlainPostgres(t *testing.T) {
	svc := NewDeploymentService(&stubDeploymentRepo{})

	if _, err := svc.CreateDeployment(context.Background(), validCreateDeploymentRequest()); err == nil {
		t.Error("CreateDeployment: expected an error on the plain Postgres data source")
	}
	if _, err := svc.UpdateDeployment(context.Background(), domain.UpdateDeploymentRequest{ID: testDeploymentUUID}); err == nil {
		t.Error("UpdateDeployment: expected an error on the plain Postgres data source")
	}
}

// If ServiceNow never accepts the deployment, nothing must be written to
// Postgres at all -- stubDeploymentRepo panics if
// CreateDeploymentFromServiceNow is called, which is exactly the assertion.
func TestDeploymentService_CreateDeployment_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorDeploymentService{
		createDeploymentSNFirstDetailsFn: func(context.Context, domain.CreateDeploymentRequest) (string, string, string, time.Time, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return "", "", "", time.Time{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubDeploymentRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeploymentServiceWithSNWriteback(repo, dispatcher, mirror)

	_, err := svc.CreateDeployment(context.Background(), validCreateDeploymentRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the deployment")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// On ServiceNow success, the Postgres insert must use EXACTLY the
// id/number/createdBy/createdOn ServiceNow returned -- not anything
// generated locally -- so both systems agree on identity from the moment
// the Postgres row exists.
func TestDeploymentService_CreateDeployment_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID        = "33333333-3333-3333-3333-333333333333"
		snNumber    = "DEP000099001"
		snCreatedBy = "jane.doe@example.com"
	)
	createdOn := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mirror := &stubMirrorDeploymentService{
		createDeploymentSNFirstDetailsFn: func(context.Context, domain.CreateDeploymentRequest) (string, string, string, time.Time, error) {
			return snID, snNumber, snCreatedBy, createdOn, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotCreatedBy string
	var gotCreatedOn time.Time
	repo := &stubDeploymentRepo{
		createDeploymentFromServiceNow: func(_ context.Context, req domain.CreateDeploymentRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployment, error) {
			mu.Lock()
			gotID, gotNumber, gotCreatedBy, gotCreatedOn = id, number, createdBy, createdOn
			mu.Unlock()
			return domain.CreatedDeployment{ID: id, CreatedBy: createdBy, CreatedOn: createdOn}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeploymentServiceWithSNWriteback(repo, dispatcher, mirror)

	resp, err := svc.CreateDeployment(context.Background(), validCreateDeploymentRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotCreatedBy != snCreatedBy || !gotCreatedOn.Equal(createdOn) {
		t.Errorf("CreateDeploymentFromServiceNow got (%q, %q, %q, %v), want (%q, %q, %q, %v)",
			gotID, gotNumber, gotCreatedBy, gotCreatedOn, snID, snNumber, snCreatedBy, createdOn)
	}
	if resp.Deployment.ID != snID {
		t.Errorf("response Deployment.ID = %q, want %q", resp.Deployment.ID, snID)
	}
}

// UpdateDeployment writes Postgres first and dispatches the ServiceNow
// mirror asynchronously afterward -- the mirror call must still happen, just
// not block the response.
func TestDeploymentService_UpdateDeployment_PostgresFirstThenAsyncMirror(t *testing.T) {
	name := "Renamed"
	req := domain.UpdateDeploymentRequest{ID: testDeploymentUUID, Name: &name}

	called := make(chan struct{})
	var mu sync.Mutex
	var gotReq domain.UpdateDeploymentRequest
	mirror := &stubMirrorDeploymentService{
		updateDeployment: func(_ context.Context, r domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
			mu.Lock()
			gotReq = r
			mu.Unlock()
			close(called)
			return domain.UpdateDeploymentResponse{}, nil
		},
	}
	repo := &stubDeploymentRepo{
		updateDeploymentFields: func(_ context.Context, r domain.UpdateDeploymentRequest, updatedBy string) (domain.UpdatedDeployment, error) {
			return domain.UpdatedDeployment{ID: r.ID, UpdatedBy: updatedBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeploymentServiceWithSNWriteback(repo, dispatcher, mirror)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateDeployment(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateDeployment was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotReq.ID != req.ID || gotReq.Name == nil || *gotReq.Name != name {
		t.Errorf("mirror got %+v, want ID=%q Name=%q", gotReq, req.ID, name)
	}
}

// A failed async mirror write must not fail the update itself -- Postgres
// already committed, and the caller already has a 200 by the time the
// mirror even runs.
func TestDeploymentService_UpdateDeployment_MirrorFailureDoesNotFailTheUpdate(t *testing.T) {
	name := "Renamed"
	req := domain.UpdateDeploymentRequest{ID: testDeploymentUUID, Name: &name}

	called := make(chan struct{})
	mirror := &stubMirrorDeploymentService{
		updateDeployment: func(context.Context, domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
			close(called)
			return domain.UpdateDeploymentResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubDeploymentRepo{
		updateDeploymentFields: func(_ context.Context, r domain.UpdateDeploymentRequest, updatedBy string) (domain.UpdatedDeployment, error) {
			return domain.UpdatedDeployment{ID: r.ID, UpdatedBy: updatedBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeploymentServiceWithSNWriteback(repo, dispatcher, mirror)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateDeployment(ctx, req); err != nil {
		t.Fatalf("unexpected error from UpdateDeployment despite Postgres succeeding: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateDeployment was never called")
	}
}
