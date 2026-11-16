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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

type fakeCaseEscalationAccessClient struct {
	got string
	err error
}

func (f *fakeCaseEscalationAccessClient) ListCaseDeescalators(context.Context) ([]byte, error) {
	f.got = "list"
	return []byte(`{"deescalators":[]}`), f.err
}

func (f *fakeCaseEscalationAccessClient) PutCaseDeescalator(_ context.Context, user string) ([]byte, error) {
	f.got = "put " + user
	return nil, f.err
}

func (f *fakeCaseEscalationAccessClient) DeleteCaseDeescalator(_ context.Context, user string) ([]byte, error) {
	f.got = "delete " + user
	return nil, f.err
}

const deescalatorTestUser = "44444444-4444-4444-8444-444444444444"

func deescalatorRequest(method, user string) *http.Request {
	r := httptest.NewRequest(method, "/case-escalation-access/deescalators/"+user, nil)
	r.SetPathValue("userId", user)
	return r
}

func TestCaseEscalationAccess_ListsAndForwardsAGrantOrRemoval(t *testing.T) {
	fake := &fakeCaseEscalationAccessClient{}
	h := NewCaseEscalationAccessHandler(fake)

	w := httptest.NewRecorder()
	h.ListDeescalators(w, withUser(httptest.NewRequest(http.MethodGet, "/case-escalation-access/deescalators", nil)))
	if w.Code != http.StatusOK || fake.got != "list" {
		t.Fatalf("list: status %d, forwarded %q", w.Code, fake.got)
	}

	w = httptest.NewRecorder()
	h.PutDeescalator(w, withUser(deescalatorRequest(http.MethodPut, deescalatorTestUser)))
	if w.Code != http.StatusNoContent || fake.got != "put "+deescalatorTestUser {
		t.Fatalf("put: status %d, forwarded %q", w.Code, fake.got)
	}

	w = httptest.NewRecorder()
	h.DeleteDeescalator(w, withUser(deescalatorRequest(http.MethodDelete, deescalatorTestUser)))
	if w.Code != http.StatusNoContent || fake.got != "delete "+deescalatorTestUser {
		t.Fatalf("delete: status %d, forwarded %q", w.Code, fake.got)
	}
}

func TestCaseEscalationAccess_RefusesABadIdOrNoUserWithoutCallingOut(t *testing.T) {
	fake := &fakeCaseEscalationAccessClient{}
	h := NewCaseEscalationAccessHandler(fake)

	w := httptest.NewRecorder()
	h.PutDeescalator(w, withUser(deescalatorRequest(http.MethodPut, "someone")))
	if w.Code != http.StatusBadRequest || fake.got != "" {
		t.Fatalf("bad id: status %d, forwarded %q", w.Code, fake.got)
	}

	w = httptest.NewRecorder()
	h.PutDeescalator(w, deescalatorRequest(http.MethodPut, deescalatorTestUser))
	if w.Code != http.StatusUnauthorized || fake.got != "" {
		t.Fatalf("no user: status %d, forwarded %q", w.Code, fake.got)
	}
}

func TestCaseEscalationAccess_AnswersARefusalWithAFixedMessage(t *testing.T) {
	fake := &fakeCaseEscalationAccessClient{err: &apierror.Error{StatusCode: http.StatusForbidden, Body: `{"error":"only a CSM admin can change who may de-escalate"}`}}
	h := NewCaseEscalationAccessHandler(fake)
	w := httptest.NewRecorder()
	h.PutDeescalator(w, withUser(deescalatorRequest(http.MethodPut, deescalatorTestUser)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", w.Code)
	}
	assertErrorMessage(t, w, ErrMsgForbidden)
	if strings.Contains(w.Body.String(), "CSM admin") {
		t.Fatalf("entity-service's own text leaked: %s", w.Body.String())
	}
}
