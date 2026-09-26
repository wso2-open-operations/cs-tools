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

type stubSLAStatusRepo struct {
	search func(ctx context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error)
}

func (s stubSLAStatusRepo) SearchActiveSLAStatuses(ctx context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
	return s.search(ctx, p)
}

// restrictedAccess is an AccessService stub whose scope is never Unrestricted
// -- an authenticated-but-non-internal caller (e.g. a project_contact's own
// x-user-id-token), as opposed to no caller identity at all.
type restrictedAccess struct{}

func (restrictedAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{ProjectIDs: []string{"proj-1"}}, nil
}

// erroringAccess is an AccessService stub whose ResolveScope always fails --
// e.g. no verified identity on the request at all (see AccessService.
// ResolveScope's own doc comment for the unverified-identity case).
type erroringAccess struct{ err error }

func (e erroringAccess) ResolveScope(context.Context) (AccessScope, error) {
	return AccessScope{}, e.err
}

// TestSLAStatusService_SearchActiveSLAStatuses_RequiresInternalCaller is the
// regression guard for a real finding: this endpoint returns every active
// case's SLA data (case number, title, product, severity) in one bulk list
// with no per-project/per-case filtering of its own, so unlike every other
// Postgres-backed read, there is no scope short of "internal service" that's
// safe to hand this out under -- a caller with no token at all, or a
// correctly-authenticated but non-internal one, must both be refused before
// the repository is ever reached.
func TestSLAStatusService_SearchActiveSLAStatuses_RequiresInternalCaller(t *testing.T) {
	repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination) ([]domain.SLAStatus, int, error) {
		t.Fatal("repository must not be reached for a non-internal caller")
		return nil, 0, nil
	}}

	t.Run("no verified identity on the request is refused", func(t *testing.T) {
		_, err := NewSLAStatusService(repo, erroringAccess{err: &apierror.ServiceUnavailableError{Msg: "no verified identity"}}).
			SearchActiveSLAStatuses(context.Background(), domain.Pagination{})
		if err == nil {
			t.Fatal("err = nil, want the AccessService error propagated")
		}
	})

	t.Run("an authenticated but non-internal caller is refused", func(t *testing.T) {
		_, err := NewSLAStatusService(repo, restrictedAccess{}).SearchActiveSLAStatuses(context.Background(), domain.Pagination{})
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("err = %v, want *apierror.ForbiddenError", err)
		}
	})
}

// TestSLAStatusService_SearchActiveSLAStatuses proves the pagination default
// and cap are this endpoint's own (500/2000), not the generic 20/50 every
// other search uses — this is a machine-polling endpoint with one real
// caller, not a UI list.
func TestSLAStatusService_SearchActiveSLAStatuses(t *testing.T) {
	t.Run("no limit defaults to 500", func(t *testing.T) {
		var got domain.Pagination
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
			got = p
			return nil, 0, nil
		}}
		if _, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}); err != nil {
			t.Fatal(err)
		}
		if got.Limit != defaultSLAStatusLimit {
			t.Errorf("limit = %d, want %d", got.Limit, defaultSLAStatusLimit)
		}
	})

	t.Run("limit above 2000 is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination) ([]domain.SLAStatus, int, error) {
			t.Fatal("repository must not be reached for an invalid limit")
			return nil, 0, nil
		}}
		_, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 2001})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("result is passed through with the normalized pagination echoed back", func(t *testing.T) {
		want := []domain.SLAStatus{{CaseID: "c1", ClockType: "response"}}
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
			return want, 7, nil
		}}
		resp, err := NewSLAStatusService(repo, alwaysUnrestrictedAccess{}).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 10, Offset: 20})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 7 || resp.Limit != 10 || resp.Offset != 20 || len(resp.Statuses) != 1 || resp.Statuses[0].CaseID != "c1" {
			t.Errorf("resp = %+v", resp)
		}
	})
}
