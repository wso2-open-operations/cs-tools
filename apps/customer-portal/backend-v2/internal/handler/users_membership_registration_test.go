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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/scim"
)

// fakeFirstAccessUserClient is an entityUserClient that records whether the
// cutover-only FirstAccess call was made, and signals when it happens so a
// test can wait for the goroutine GetMe starts. When rec is set it also
// records how much of the response had been written at the moment the call
// started, which is what proves the ordering.
type fakeFirstAccessUserClient struct {
	err           error
	called        chan struct{}
	rec           *httptest.ResponseRecorder
	bodyLenAtCall int
}

func (f *fakeFirstAccessUserClient) GetMe(context.Context) (entity.GetUserMeResponse, error) {
	return entity.GetUserMeResponse{ID: "u-1", Email: "jane@acme.com", LastName: "Doe", Roles: []string{"customer_admin"}}, nil
}

func (f *fakeFirstAccessUserClient) PatchMe(context.Context, entity.PatchUserMeRequest) (entity.PatchUserMeResponse, error) {
	return entity.PatchUserMeResponse{}, nil
}

func (f *fakeFirstAccessUserClient) RegisterInvitedMemberships(context.Context) error {
	if f.rec != nil {
		f.bodyLenAtCall = f.rec.Body.Len()
	}
	close(f.called)
	return f.err
}

// noopSCIMUserClient keeps the profile's phone-number lookup out of the way:
// these tests are about the onboarding call, not about SCIM.
type noopSCIMUserClient struct{}

func (noopSCIMUserClient) SearchUser(context.Context, string) (*scim.UserInfo, error) {
	return nil, nil
}

func (noopSCIMUserClient) UpdateUserPhone(context.Context, string, string) (*string, error) {
	return nil, nil
}

func getMeRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	return req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "u-1", Email: "jane@acme.com"}))
}

// TestGetMe_FirstAccessDisabledMakesNoCall is the guarantee that matters
// before cutover: with CSM_MIGRATION_FIRST_ACCESS_ENABLED unset, GetMe must
// behave exactly as it always has and must never reach entity-service's
// onboarding route.
func TestGetMe_FirstAccessDisabledMakesNoCall(t *testing.T) {
	client := &fakeFirstAccessUserClient{called: make(chan struct{})}
	h := NewUserHandler(client, noopSCIMUserClient{}, false)

	rec := httptest.NewRecorder()
	h.GetMe(rec, getMeRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	select {
	case <-client.called:
		t.Fatal("RegisterInvitedMemberships was called with the flag off")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestGetMe_FirstAccessEnabledCallsAfterResponding: with the flag on the
// call happens, and it happens after the profile has been written, so it
// cannot delay or alter what the browser receives.
func TestGetMe_FirstAccessEnabledCallsAfterResponding(t *testing.T) {
	rec := httptest.NewRecorder()
	client := &fakeFirstAccessUserClient{called: make(chan struct{}), rec: rec}
	h := NewUserHandler(client, noopSCIMUserClient{}, true)

	h.GetMe(rec, getMeRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if body["email"] != "jane@acme.com" {
		t.Errorf("profile = %v, want the usual body regardless of the onboarding call", body)
	}

	select {
	case <-client.called:
	case <-time.After(2 * time.Second):
		t.Fatal("RegisterInvitedMemberships was not called with the flag on")
	}
	// Read after the receive above, which orders it after the fake's write.
	if client.bodyLenAtCall == 0 {
		t.Error("RegisterInvitedMemberships started before the profile was written")
	}
}

// TestGetMe_FirstAccessFailureDoesNotAffectTheResponse: entity-service being
// down, or not having the route at all (it is registered only when its own
// cutover flag is on), must be invisible to the signed-in user.
func TestGetMe_FirstAccessFailureDoesNotAffectTheResponse(t *testing.T) {
	client := &fakeFirstAccessUserClient{called: make(chan struct{}), err: errors.New("entity-service unavailable")}
	h := NewUserHandler(client, noopSCIMUserClient{}, true)

	rec := httptest.NewRecorder()
	h.GetMe(rec, getMeRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite the onboarding call failing", rec.Code)
	}
	select {
	case <-client.called:
	case <-time.After(2 * time.Second):
		t.Fatal("RegisterInvitedMemberships was not called")
	}
	if rec.Body.Len() == 0 {
		t.Error("profile body must still be written")
	}
}
