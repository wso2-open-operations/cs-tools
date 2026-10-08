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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeOutageRepo records Create calls. Embedding the interface leaves every
// other method nil: touching one would panic, which is the point.
type fakeOutageRepo struct {
	repository.OutageRepository
	creates   []repository.OutageWrite
	createErr error
}

func (f *fakeOutageRepo) PublicationFor(context.Context, *string) (bool, *string, error) {
	return false, nil, nil
}

func (f *fakeOutageRepo) Create(_ context.Context, in repository.OutageWrite) (domain.Outage, error) {
	f.creates = append(f.creates, in)
	if f.createErr != nil {
		return domain.Outage{}, f.createErr
	}
	n, id := in.Number, in.ID
	if n == "" {
		n, id = "OUT0010000", "native-id"
	}
	return domain.Outage{ID: id, Number: n, ShortDescription: in.ShortDescription}, nil
}

// fakeExternalOutage stands in for the external-system service.
type fakeExternalOutage struct {
	OutageService
	calls int
	resp  domain.CreateOutageResponse
	err   error
}

func (f *fakeExternalOutage) CreateOutage(context.Context, domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	f.calls++
	return f.resp, f.err
}

func validOutageReq() domain.CreateOutageRequest {
	return domain.CreateOutageRequest{
		Type:             domain.OutageTypeOutage,
		Begin:            "2026-01-02 03:04:05",
		ShortDescription: "Example outage",
	}
}

const extOutageID = "0123456789abcdef0123456789abcdef"

func TestOutageSNFirst_CreateCallsExternalFirstAndStoresItsNumberAndID(t *testing.T) {
	repo := &fakeOutageRepo{}
	ext := &fakeExternalOutage{resp: domain.CreateOutageResponse{Outage: domain.Outage{
		ID: sysidToUUID(extOutageID), Number: "OUT0001888"}}}
	svc := NewOutageServiceWithSNFirstCreate(repo, ext)

	resp, err := svc.CreateOutage(context.Background(), validOutageReq())
	if err != nil {
		t.Fatalf("CreateOutage: %v", err)
	}
	if ext.calls != 1 {
		t.Errorf("external create calls = %d, want 1", ext.calls)
	}
	if len(repo.creates) != 1 {
		t.Fatalf("repo creates = %d, want 1", len(repo.creates))
	}
	got := repo.creates[0]
	if got.Number != "OUT0001888" || got.ID != "01234567-89ab-cdef-0123-456789abcdef" {
		t.Errorf("repo got number=%q id=%q, want the external ones in uuid form", got.Number, got.ID)
	}
	if resp.Outage.Number != "OUT0001888" || resp.Message != "Outage created successfully." {
		t.Errorf("response = %+v, want the Postgres read-back", resp)
	}
}

func TestOutageSNFirst_ExternalFailureWritesNothingAndIsNotRetried(t *testing.T) {
	boom := errors.New("upstream down")
	repo := &fakeOutageRepo{}
	ext := &fakeExternalOutage{err: boom}
	svc := NewOutageServiceWithSNFirstCreate(repo, ext)

	_, err := svc.CreateOutage(context.Background(), validOutageReq())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the external error", err)
	}
	if ext.calls != 1 {
		t.Errorf("external create calls = %d, want exactly 1 (no retry)", ext.calls)
	}
	if len(repo.creates) != 0 {
		t.Errorf("repo must not be called, got %d creates", len(repo.creates))
	}
}

func TestOutageSNFirst_PostgresFailureAfterExternalSuccessReturnsError(t *testing.T) {
	boom := errors.New("insert failed")
	repo := &fakeOutageRepo{createErr: boom}
	ext := &fakeExternalOutage{resp: domain.CreateOutageResponse{Outage: domain.Outage{
		ID: sysidToUUID(extOutageID), Number: "OUT0001888"}}}
	svc := NewOutageServiceWithSNFirstCreate(repo, ext)

	_, err := svc.CreateOutage(context.Background(), validOutageReq())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the Postgres error", err)
	}
	if ext.calls != 1 || len(repo.creates) != 1 {
		t.Errorf("external calls=%d repo creates=%d, want 1 and 1", ext.calls, len(repo.creates))
	}
}

func TestOutageSNFirst_InvalidRequestNeverReachesExternal(t *testing.T) {
	repo := &fakeOutageRepo{}
	ext := &fakeExternalOutage{}
	svc := NewOutageServiceWithSNFirstCreate(repo, ext)

	req := validOutageReq()
	req.ShortDescription = ""
	_, err := svc.CreateOutage(context.Background(), req)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != "shortDescription is required" {
		t.Fatalf("err = %v, want the shared validation message", err)
	}
	if ext.calls != 0 || len(repo.creates) != 0 {
		t.Errorf("external calls=%d repo creates=%d, want 0 and 0", ext.calls, len(repo.creates))
	}
}

func TestOutageSNFirst_ReplyWithoutNumberWritesNothing(t *testing.T) {
	repo := &fakeOutageRepo{}
	ext := &fakeExternalOutage{resp: domain.CreateOutageResponse{Outage: domain.Outage{ID: sysidToUUID(extOutageID)}}}
	svc := NewOutageServiceWithSNFirstCreate(repo, ext)

	_, err := svc.CreateOutage(context.Background(), validOutageReq())
	var de *apierror.DownstreamError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, want a DownstreamError", err)
	}
	if len(repo.creates) != 0 {
		t.Errorf("repo must not be called, got %d creates", len(repo.creates))
	}
}

func TestOutagePostgresOnly_CreateUsesNativePathAndNeverCallsExternal(t *testing.T) {
	repo := &fakeOutageRepo{}
	svc := NewOutageService(repo)

	resp, err := svc.CreateOutage(context.Background(), validOutageReq())
	if err != nil {
		t.Fatalf("CreateOutage: %v", err)
	}
	if len(repo.creates) != 1 {
		t.Fatalf("repo creates = %d, want 1", len(repo.creates))
	}
	if repo.creates[0].Number != "" || repo.creates[0].ID != "" {
		t.Errorf("native path must leave Number/ID unset, got %q/%q", repo.creates[0].Number, repo.creates[0].ID)
	}
	if resp.Outage.Number != "OUT0010000" {
		t.Errorf("number = %q, want the native one", resp.Outage.Number)
	}
}
