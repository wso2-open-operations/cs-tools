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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// recordingFeatureOverrideRepo keeps rows in a map, the way the table keys them
// by nav_node_id, so a test can see an upsert reuse a row and nothing delete one.
type recordingFeatureOverrideRepo struct {
	rows      map[string]domain.FeatureOverride
	savedOnce bool
	calls     int
	actor     string
	lastReq   domain.SaveFeatureOverrideRequest
}

func newRecordingFeatureOverrideRepo() *recordingFeatureOverrideRepo {
	return &recordingFeatureOverrideRepo{rows: map[string]domain.FeatureOverride{}}
}

func (r *recordingFeatureOverrideRepo) List(context.Context) (domain.FeatureOverrideList, error) {
	out := []domain.FeatureOverride{}
	for _, row := range r.rows {
		out = append(out, row)
	}
	return domain.FeatureOverrideList{Overrides: out, SavedOnce: r.savedOnce}, nil
}

func (r *recordingFeatureOverrideRepo) Put(_ context.Context, actor string, req domain.SaveFeatureOverrideRequest) (domain.FeatureOverride, error) {
	r.calls++
	r.actor = actor
	r.lastReq = req
	row := domain.FeatureOverride{NavNodeID: req.NavNodeID, State: req.State, UpdatedBy: actor}
	r.rows[req.NavNodeID] = row
	if req.MarkSaved {
		r.savedOnce = true
	}
	return row, nil
}

func newTestFeatureOverrideService(repo *recordingFeatureOverrideRepo) FeatureOverrideService {
	return NewFeatureOverrideService(repo, alwaysUnrestrictedAccess{}, testCSMPortalBackendClientID)
}

func TestFeatureOverridePutStoresPreviewAndEnabled(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	svc := newTestFeatureOverrideService(repo)
	for _, state := range []string{domain.FeaturePreview, domain.FeatureEnabled} {
		got, err := svc.Put(bannerCSMPortalCtx(), domain.SaveFeatureOverrideRequest{NavNodeID: "operations", State: state})
		if err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		if got.State != state || repo.actor != "admin@wso2.com" {
			t.Fatalf("%s: stored %+v by %s", state, got, repo.actor)
		}
	}
}

func TestFeatureOverrideSecondPutOfEnabledKeepsTheSameRow(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	svc := newTestFeatureOverrideService(repo)
	ctx := bannerCSMPortalCtx()
	if _, err := svc.Put(ctx, domain.SaveFeatureOverrideRequest{NavNodeID: "operations.incidents", State: "preview"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Put(ctx, domain.SaveFeatureOverrideRequest{NavNodeID: "operations.incidents", State: "enabled"}); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 1 || repo.rows["operations.incidents"].State != "enabled" {
		t.Fatalf("rows = %+v", repo.rows)
	}
	// Setting a page back to available must leave the row listed, as enabled.
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Overrides) != 1 || list.Overrides[0].State != "enabled" {
		t.Fatalf("list = %+v", list)
	}
}

func TestFeatureOverridePutRejectsABadSave(t *testing.T) {
	cases := map[string]domain.SaveFeatureOverrideRequest{
		"wip state":      {NavNodeID: "operations", State: "wip"},
		"hidden state":   {NavNodeID: "operations", State: "hidden"},
		"empty state":    {NavNodeID: "operations", State: ""},
		"blank id":       {NavNodeID: "   ", State: "enabled"},
		"id too long":    {NavNodeID: strings.Repeat("a", maxFeatureNavNodeIDLen+1), State: "enabled"},
		"upper-case id":  {NavNodeID: "Operations", State: "enabled"},
		"id with spaces": {NavNodeID: "operations incidents", State: "enabled"},
		"id with slash":  {NavNodeID: "operations/incidents", State: "enabled"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newRecordingFeatureOverrideRepo()
			_, err := newTestFeatureOverrideService(repo).Put(bannerCSMPortalCtx(), req)
			var invalid *apierror.ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v", err)
			}
			if repo.calls != 0 {
				t.Fatal("repo was called")
			}
		})
	}
}

func TestFeatureOverridePutPassesMarkSavedThrough(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	svc := newTestFeatureOverrideService(repo)
	if _, err := svc.Put(bannerCSMPortalCtx(), domain.SaveFeatureOverrideRequest{NavNodeID: "operations", State: "preview"}); err != nil {
		t.Fatal(err)
	}
	if repo.savedOnce {
		t.Fatal("mark set without markSaved")
	}
	if _, err := svc.Put(bannerCSMPortalCtx(), domain.SaveFeatureOverrideRequest{NavNodeID: "customers", State: "preview", MarkSaved: true}); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !list.SavedOnce {
		t.Fatal("markSaved was not passed to the repository")
	}
}

func TestFeatureOverridePutRequiresTheCSMPortalBackendClientID(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		Validated: true, UserEmail: "admin@wso2.com", ClientID: "some-other-client",
	})
	_, err := newTestFeatureOverrideService(repo).Put(ctx, domain.SaveFeatureOverrideRequest{NavNodeID: "operations", State: "preview"})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || repo.calls != 0 {
		t.Fatalf("err = %v, calls = %d", err, repo.calls)
	}
}

func TestFeatureOverridePutRequiresASignedInPerson(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: testCSMPortalBackendClientID})
	_, err := newTestFeatureOverrideService(repo).Put(ctx, domain.SaveFeatureOverrideRequest{NavNodeID: "operations", State: "preview"})
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) || repo.calls != 0 {
		t.Fatalf("err = %v, calls = %d", err, repo.calls)
	}
}

func TestFeatureOverrideListNeedsNoAdminRole(t *testing.T) {
	repo := newRecordingFeatureOverrideRepo()
	repo.rows["operations"] = domain.FeatureOverride{NavNodeID: "operations", State: "preview"}
	// An internal caller that is not csm-portal-backend and holds no admin role can still read.
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "engineer@wso2.com", ClientID: "some-other-client"})
	got, err := newTestFeatureOverrideService(repo).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Overrides) != 1 || got.SavedOnce {
		t.Fatalf("list = %+v", got)
	}
}
