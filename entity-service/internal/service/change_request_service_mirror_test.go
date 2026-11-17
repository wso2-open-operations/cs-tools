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
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// crMirrorSentinel is the title of the PATCH awaitMirroredWrites sends.
const crMirrorSentinel = "mirror sentinel"

// previousSystemCRServer stands in for the previous system's change request
// PATCH. It records every body it receives and signals sentinel when the
// sentinel's arrives.
type previousSystemCRServer struct {
	mu       sync.Mutex
	bodies   []map[string]any
	sentinel chan struct{}
}

func (s *previousSystemCRServer) received() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.bodies...)
}

func newPreviousSystemCRMirror(t *testing.T) (ChangeRequestService, *previousSystemCRServer) {
	t.Helper()
	srv := &previousSystemCRServer{sentinel: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/change-requests/"+uuidToSysid(testUUID), func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		srv.mu.Lock()
		srv.bodies = append(srv.bodies, body)
		srv.mu.Unlock()
		if body["title"] == crMirrorSentinel {
			srv.sentinel <- struct{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "Change request updated", "changeRequest": {"id": "` + uuidToSysid(testUUID) + `", "number": "CHG0001", "createdOn": "2026-01-01 00:00:00"}}`))
	})
	return NewServiceNowChangeRequestService(newTestSNClient(t, mux)), srv
}

// awaitMirroredWrites sends svc a PATCH of testUUID that is mirrored (a title)
// and waits until arrived reports it. The write-back dispatcher runs one change
// request's writes one at a time, in the order they were dispatched, so every
// write dispatched before the sentinel has finished, and any failure of it has
// been recorded, by the time the sentinel arrives.
func awaitMirroredWrites(ctx context.Context, t *testing.T, svc ChangeRequestService, arrived <-chan struct{}) {
	t.Helper()
	title := crMirrorSentinel
	if _, err := svc.PatchChangeRequest(ctx, testUUID, domain.PatchChangeRequestRequest{Title: &title}); err != nil {
		t.Fatalf("sentinel PATCH refused: %v", err)
	}
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the sentinel PATCH never reached the previous system")
	}
}

// The hold and the service have no field in the previous system's PATCH, and
// its adapter refuses a request that names them. A dual-write PATCH that carries
// only them is therefore not dispatched at all: nothing is sent and no
// write-back failure is recorded.
func TestChangeRequestService_PatchChangeRequest_PostgresOnlyFieldsAreNotMirrored(t *testing.T) {
	reason := "waiting for the customer's maintenance window"
	onHold, offHold := true, false
	svcID := "66666666-7777-8888-9999-000000000000"
	repo := &stubChangeRequestRepo{
		patchChangeRequest: func(_ context.Context, id string, _ domain.PatchChangeRequestRequest, _ string) (domain.ChangeRequest, error) {
			return domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}, nil
		},
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	for name, req := range map[string]domain.PatchChangeRequestRequest{
		"put on hold":          {OnHold: &onHold, OnHoldReason: &reason},
		"take off hold":        {OnHold: &offHold},
		"edit the hold reason": {OnHoldReason: &reason},
		"set the service":      {ServiceID: &svcID},
		"set the offering":     {ServiceOfferingID: &svcID},
	} {
		t.Run(name, func(t *testing.T) {
			mirror, server := newPreviousSystemCRMirror(t)
			failures := &recordingSNWritebackFailures{}
			svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
			if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
				t.Fatalf("PATCH refused: %v", err)
			}
			awaitMirroredWrites(ctx, t, svc, server.sentinel)
			if got := failures.count(); got != 0 {
				t.Errorf("write-back failures = %d, want 0", got)
			}
			if got := server.received(); len(got) != 1 {
				t.Errorf("the previous system received %v, want only the sentinel", got)
			}
		})
	}

	t.Run("with a mirrored field: only that field goes out", func(t *testing.T) {
		mirror, server := newPreviousSystemCRMirror(t)
		failures := &recordingSNWritebackFailures{}
		svc := NewChangeRequestServiceWithSNWriteback(repo, stubUserRepo{}, mirror, NewSNWritebackDispatcher(failures))
		title := crMirrorSentinel
		req := domain.PatchChangeRequestRequest{Title: &title, OnHold: &onHold, OnHoldReason: &reason, ServiceID: &svcID, ServiceOfferingID: &svcID}
		if _, err := svc.PatchChangeRequest(ctx, testUUID, req); err != nil {
			t.Fatalf("PATCH refused: %v", err)
		}
		select {
		case <-server.sentinel:
		case <-time.After(5 * time.Second):
			t.Fatal("the PATCH never reached the previous system")
		}
		got := server.received()
		if len(got) != 1 {
			t.Fatalf("the previous system received %d PATCHes, want 1", len(got))
		}
		for _, k := range []string{"onHold", "onHoldReason", "serviceId", "serviceOfferingId"} {
			if _, ok := got[0][k]; ok {
				t.Errorf("mirrored body carries %q: %v", k, got[0])
			}
		}
		// The PATCH under test signalled before its write returned, so its failure
		// (if any) may not be recorded yet. A second sentinel, run after it in
		// dispatch order, makes sure it is.
		awaitMirroredWrites(ctx, t, svc, server.sentinel)
		if n := failures.count(); n != 0 {
			t.Errorf("write-back failures = %d, want 0", n)
		}
	})
}
