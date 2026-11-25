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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func incidentDeltaCtx(t *testing.T) context.Context {
	t.Helper()
	return contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
}

// The delta reaches the repository as a delta, never as a replace list: that
// is what keeps a stale caller from dropping a watcher someone else added.
func TestIncidentService_UpdateIncident_WatchDelta_SendsDeltaNotReplace(t *testing.T) {
	var got repository.IncidentLifecycleUpdate
	view := newTestIncidentView(testDeploymentUUID)
	repo := &stubIncidentRepo{
		updateIncidentLifecycle: func(_ context.Context, _ string, u repository.IncidentLifecycleUpdate, _ string) error {
			got = u
			return nil
		},
		getIncidentByID: func(context.Context, string) (domain.IncidentView, error) { return view, nil },
	}
	svc := NewIncidentServiceWithPublisher(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, nil)

	_, err := svc.UpdateIncident(incidentDeltaCtx(t), domain.UpdateIncidentRequest{
		ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, RemoveWatchers: []string{deltaUserB},
	})
	if err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}
	if len(got.AddWatchers) != 1 || got.AddWatchers[0] != deltaUserA || len(got.RemoveWatchers) != 1 || got.RemoveWatchers[0] != deltaUserB {
		t.Errorf("repo got add=%v remove=%v", got.AddWatchers, got.RemoveWatchers)
	}
	if got.WatchList != nil {
		t.Errorf("WatchList = %v, want nil: a whole list would replace the stored one", *got.WatchList)
	}
}

func TestIncidentService_UpdateIncident_WatchDelta_Refusals(t *testing.T) {
	list := []string{deltaUserA}
	cases := map[string]domain.UpdateIncidentRequest{
		"same user in both lists": {ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, RemoveWatchers: []string{deltaUserA}},
		"not a uuid":              {ID: testDeploymentUUID, RemoveWatchers: []string{"not-a-uuid"}},
		"with watchList":          {ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, WatchList: &list},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &stubIncidentRepo{} // panics if the write is reached
			svc := NewIncidentServiceWithPublisher(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, nil)
			_, err := svc.UpdateIncident(incidentDeltaCtx(t), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
		})
	}
}

// In dual-write ServiceNow can only replace its watch list, so the mirror gets
// the list Postgres holds after the change, not the delta.
func TestIncidentService_UpdateIncident_WatchDelta_MirrorGetsTheResultingList(t *testing.T) {
	after := newTestIncidentView(testDeploymentUUID)
	after.WatchList = []domain.IncidentWatchListItem{{ID: deltaUserA}, {ID: deltaUserB}}
	repo := &stubIncidentRepo{
		updateIncidentLifecycle: func(context.Context, string, repository.IncidentLifecycleUpdate, string) error { return nil },
		getIncidentByID:         func(context.Context, string) (domain.IncidentView, error) { return after, nil },
	}
	mirrored := make(chan domain.UpdateIncidentRequest, 1)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrored <- req
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewIncidentServiceWithSNMirror(repo, stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}, mirror, nil, dispatcher)

	if _, err := svc.UpdateIncident(incidentDeltaCtx(t), domain.UpdateIncidentRequest{
		ID: testDeploymentUUID, AddWatchers: []string{deltaUserB},
	}); err != nil {
		t.Fatalf("UpdateIncident: %v", err)
	}
	select {
	case req := <-mirrored:
		if len(req.AddWatchers) != 0 || len(req.RemoveWatchers) != 0 {
			t.Errorf("mirror got a delta (%v, %v), which ServiceNow cannot apply", req.AddWatchers, req.RemoveWatchers)
		}
		if req.WatchList == nil || len(*req.WatchList) != 2 || (*req.WatchList)[0] != deltaUserA || (*req.WatchList)[1] != deltaUserB {
			t.Errorf("mirror WatchList = %v, want both watchers", req.WatchList)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the update was never mirrored")
	}
}

// The ServiceNow data source refuses the delta rather than emulating it.
func TestSNIncidentService_UpdateIncident_RefusesWatchDelta(t *testing.T) {
	svc := &snIncidentService{}
	_, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{
		ID: testDeploymentUUID, AddWatchers: []string{deltaUserA},
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected a ValidationError, got %v", err)
	}
}
