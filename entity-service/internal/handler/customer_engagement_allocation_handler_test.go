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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type stubAllocationSvc struct {
	got    *domain.AllocationEvent
	hadSys bool
	res    domain.AllocationEventResult
	err    error
}

func (s *stubAllocationSvc) ProcessAllocationEvent(ctx context.Context, ev domain.AllocationEvent) (domain.AllocationEventResult, error) {
	s.got = &ev
	scope, ok := repository.CallerIdentityFromContext(ctx)
	s.hadSys = ok && scope.Unrestricted
	return s.res, s.err
}

func postAllocationEvent(h *CustomerEngagementAllocationHandler, clientID, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/customer-engagements/allocation-events", strings.NewReader(body))
	if clientID != "" {
		r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{Validated: true, ClientID: clientID}))
	}
	w := httptest.NewRecorder()
	h.ProcessAllocationEvent(w, r)
	return w
}

func TestAllocationEventHandler_RejectsNonInternalClient(t *testing.T) {
	svc := &stubAllocationSvc{}
	h := NewCustomerEngagementAllocationHandler(svc, map[string]bool{"finance": true})
	for _, client := range []string{"", "portal"} {
		if w := postAllocationEvent(h, client, `{"id":"A1","email":"a@b.c"}`); w.Code != http.StatusUnauthorized {
			t.Errorf("client %q: status %d", client, w.Code)
		}
	}
	if svc.got != nil {
		t.Error("service called for a non-internal caller")
	}
}

func TestAllocationEventHandler_IgnoresUnknownFieldsAndAnswersSkip(t *testing.T) {
	svc := &stubAllocationSvc{res: domain.AllocationEventResult{Result: domain.AllocationEventSkipped, Reason: "no engagement for line item"}}
	h := NewCustomerEngagementAllocationHandler(svc, map[string]bool{"finance": true})
	w := postAllocationEvent(h, "finance",
		`{"id":"A1","email":"a@b.c","allocationType":12,"addedBy":"x","somethingNew":1,"engagement":{"productId":"00k1","fundingSources":[]}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if svc.got == nil || svc.got.Engagement == nil || *svc.got.Engagement.ProductID != "00k1" || !svc.hadSys {
		t.Fatalf("service got %+v, system identity %v", svc.got, svc.hadSys)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["result"] != "skipped" || body["reason"] != "no engagement for line item" ||
		body["engagementId"] != nil || body["engagementCreated"] != false {
		t.Errorf("body = %v", body)
	}
	if _, ok := body["allocationResourceId"]; !ok {
		t.Error("allocationResourceId missing; want null")
	}
}

func TestAllocationEventHandler_ErrorStatuses(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
		want int
	}{
		{"malformed", `{"id":`, nil, http.StatusBadRequest},
		{"validation", `{"id":""}`, &apierror.ValidationError{Msg: "id and email are required"}, http.StatusBadRequest},
		{"failure", `{"id":"A1","email":"a@b.c"}`, errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewCustomerEngagementAllocationHandler(&stubAllocationSvc{err: tc.err}, map[string]bool{"finance": true})
			if w := postAllocationEvent(h, "finance", tc.body); w.Code != tc.want {
				t.Errorf("status %d, want %d", w.Code, tc.want)
			}
		})
	}
}
