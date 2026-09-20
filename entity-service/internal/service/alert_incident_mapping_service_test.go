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
)

// fakeAlertIncidentMappingRepo captures the request it was called with so
// tests can assert on what the service layer forwards to the repository,
// without a live Postgres connection.
type fakeAlertIncidentMappingRepo struct {
	gotCreateReq domain.CreateAlertIncidentMappingRequest
	createResult domain.AlertIncidentMappingView
	createErr    error

	gotLookupSource           string
	gotLookupUniqueIdentifier string
	lookupResult              []domain.AlertIncidentMappingView
	lookupErr                 error
}

func (f *fakeAlertIncidentMappingRepo) Create(_ context.Context, req domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error) {
	f.gotCreateReq = req
	return f.createResult, f.createErr
}

func (f *fakeAlertIncidentMappingRepo) Lookup(_ context.Context, source, uniqueIdentifier string) ([]domain.AlertIncidentMappingView, error) {
	f.gotLookupSource = source
	f.gotLookupUniqueIdentifier = uniqueIdentifier
	return f.lookupResult, f.lookupErr
}

func TestAlertIncidentMappingService_CreateAlertIncidentMapping_ForwardsValidRequest(t *testing.T) {
	repo := &fakeAlertIncidentMappingRepo{createResult: domain.AlertIncidentMappingView{ID: "id-1", AlertNumber: "ALT-1"}}
	svc := NewAlertIncidentMappingService(repo)

	req := domain.CreateAlertIncidentMappingRequest{
		AlertNumber: "ALT-1",
		Source:      "datadog",
		AlertStatus: "firing",
		IncidentID:  "inc-1",
	}
	got, err := svc.CreateAlertIncidentMapping(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotCreateReq.AlertNumber != "ALT-1" || repo.gotCreateReq.Source != "datadog" {
		t.Fatalf("expected request forwarded unchanged, got %+v", repo.gotCreateReq)
	}
	if got.ID != "id-1" {
		t.Fatalf("expected repo result returned unchanged, got %+v", got)
	}
}

func TestAlertIncidentMappingService_CreateAlertIncidentMapping_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		req  domain.CreateAlertIncidentMappingRequest
	}{
		{"missing alertNumber", domain.CreateAlertIncidentMappingRequest{Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1"}},
		{"missing source", domain.CreateAlertIncidentMappingRequest{AlertNumber: "ALT-1", AlertStatus: "firing", IncidentID: "inc-1"}},
		{"missing alertStatus", domain.CreateAlertIncidentMappingRequest{AlertNumber: "ALT-1", Source: "datadog", IncidentID: "inc-1"}},
		{"missing incidentId", domain.CreateAlertIncidentMappingRequest{AlertNumber: "ALT-1", Source: "datadog", AlertStatus: "firing"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeAlertIncidentMappingRepo{}
			svc := NewAlertIncidentMappingService(repo)
			_, err := svc.CreateAlertIncidentMapping(context.Background(), c.req)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected a ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestAlertIncidentMappingService_CreateAlertIncidentMapping_PropagatesConflict(t *testing.T) {
	wantErr := &apierror.ConflictError{Msg: "alertNumber is already mapped to an incident: ALT-1"}
	repo := &fakeAlertIncidentMappingRepo{createErr: wantErr}
	svc := NewAlertIncidentMappingService(repo)

	req := domain.CreateAlertIncidentMappingRequest{
		AlertNumber: "ALT-1", Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1",
	}
	_, err := svc.CreateAlertIncidentMapping(context.Background(), req)
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected a ConflictError propagated from the repo, got %T: %v", err, err)
	}
}

func TestAlertIncidentMappingService_LookupAlertIncidentMappings_RejectsMissingFields(t *testing.T) {
	repo := &fakeAlertIncidentMappingRepo{}
	svc := NewAlertIncidentMappingService(repo)

	if _, err := svc.LookupAlertIncidentMappings(context.Background(), domain.LookupAlertIncidentMappingsRequest{UniqueIdentifier: "grp-1"}); err == nil {
		t.Fatal("expected validation error for missing source, got nil")
	}
	if _, err := svc.LookupAlertIncidentMappings(context.Background(), domain.LookupAlertIncidentMappingsRequest{Source: "datadog"}); err == nil {
		t.Fatal("expected validation error for missing uniqueIdentifier, got nil")
	}
}

func TestAlertIncidentMappingService_LookupAlertIncidentMappings_ForwardsToRepo(t *testing.T) {
	repo := &fakeAlertIncidentMappingRepo{lookupResult: []domain.AlertIncidentMappingView{
		{ID: "id-1", AlertNumber: "ALT-1"},
		{ID: "id-2", AlertNumber: "ALT-2"},
	}}
	svc := NewAlertIncidentMappingService(repo)

	resp, err := svc.LookupAlertIncidentMappings(context.Background(), domain.LookupAlertIncidentMappingsRequest{
		Source: "datadog", UniqueIdentifier: "grp-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotLookupSource != "datadog" || repo.gotLookupUniqueIdentifier != "grp-1" {
		t.Fatalf("expected source/uniqueIdentifier forwarded, got %q/%q", repo.gotLookupSource, repo.gotLookupUniqueIdentifier)
	}
	if len(resp.Mappings) != 2 {
		t.Fatalf("expected 2 mappings returned unchanged, got %d", len(resp.Mappings))
	}
}

// TestAlertIncidentMappingService_LookupAlertIncidentMappings_EmptyResultIsNotAnError
// verifies that no matches is a valid, successful lookup result — an empty
// slice, not a NotFoundError — since a lookup's job is to report absence,
// not treat it as failure.
func TestAlertIncidentMappingService_LookupAlertIncidentMappings_EmptyResultIsNotAnError(t *testing.T) {
	repo := &fakeAlertIncidentMappingRepo{lookupResult: []domain.AlertIncidentMappingView{}}
	svc := NewAlertIncidentMappingService(repo)

	resp, err := svc.LookupAlertIncidentMappings(context.Background(), domain.LookupAlertIncidentMappingsRequest{
		Source: "datadog", UniqueIdentifier: "grp-unknown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Mappings == nil || len(resp.Mappings) != 0 {
		t.Fatalf("expected an empty, non-nil Mappings slice, got %+v", resp.Mappings)
	}
}
