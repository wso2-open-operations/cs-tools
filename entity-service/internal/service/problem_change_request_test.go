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
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testChangeRequestUUID = "6c3db375-1b1c-b2d0-a002-c9d3604bcb0c"

// A change request link is a problem field: written to Postgres and, in
// dual-write, mirrored to ServiceNow; "" unlinks. It counts as an update on
// its own.
func TestUpdateProblem_ChangeRequestLinkAndUnlink(t *testing.T) {
	for _, id := range []string{testChangeRequestUUID, ""} {
		var toPG *string
		mirrored := make(chan domain.UpdateProblemRequest, 1)
		repo := &stubProblemRepo{
			updateProblemFields: func(_ context.Context, req domain.UpdateProblemRequest, _ string) (time.Time, error) {
				toPG = req.ChangeRequestID
				return time.Now(), nil
			},
			getProblem: problemDetailStub,
		}
		mirror := &stubMirrorProblemService{
			updateProblem: func(_ context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
				mirrored <- req
				return domain.UpdateProblemResponse{}, nil
			},
		}
		svc := NewProblemServiceWithSNMirror(repo, mirror, NewSNWritebackDispatcher(&recordingSNWritebackFailures{}))
		if _, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, ChangeRequestID: strp(id)}); err != nil {
			t.Fatalf("%q: %v", id, err)
		}
		if toPG == nil || *toPG != id {
			t.Errorf("%q: Postgres got %v", id, toPG)
		}
		select {
		case req := <-mirrored:
			if req.ChangeRequestID == nil || *req.ChangeRequestID != id {
				t.Errorf("%q: mirror got %v", id, req.ChangeRequestID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q: never mirrored", id)
		}
	}
}

func TestUpdateProblem_ChangeRequestIDMustBeAUUID(t *testing.T) {
	for name, svc := range map[string]ProblemService{
		"postgres":   NewProblemService(&stubProblemRepo{}),
		"dual-write": NewProblemServiceWithSNMirror(&stubProblemRepo{}, &stubMirrorProblemService{}, NewSNWritebackDispatcher(&recordingSNWritebackFailures{})),
	} {
		_, err := svc.UpdateProblem(userCtxProblem(t), domain.UpdateProblemRequest{ID: testDeploymentUUID, ChangeRequestID: strp("CHG0000001")})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
	}
}

// ServiceNow gets the change request as a sys_id, and "" as-is to unlink.
func TestSNProblemService_UpdateProblem_ChangeRequestAsSysid(t *testing.T) {
	for in, want := range map[string]string{testChangeRequestUUID: uuidToSysid(testChangeRequestUUID), "": ""} {
		var body map[string]any
		mux := http.NewServeMux()
		mux.HandleFunc("/problems/"+testProblemSysid, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"Problem updated successfully","problem":{"id":"` + testProblemSysid + `"}}`))
		})
		svc := NewServiceNowProblemService(newTestSNClient(t, mux))
		if _, err := svc.UpdateProblem(contextWithUserIDToken("token"), domain.UpdateProblemRequest{ID: testProblemUUID, ChangeRequestID: strp(in)}); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got, ok := body["changeRequestId"]; !ok || got != want {
			t.Errorf("%q: ServiceNow got changeRequestId %v, want %q", in, got, want)
		}
	}
}
