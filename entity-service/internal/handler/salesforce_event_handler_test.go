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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type stubSalesforceEventService struct {
	service.SalesforceEventService
	err    error
	called bool
	req    domain.SalesforceEventRequest
}

func (s *stubSalesforceEventService) HandleEvent(_ context.Context, req domain.SalesforceEventRequest) error {
	s.called = true
	s.req = req
	return s.err
}

func TestHandleEvent_NoContent(t *testing.T) {
	svc := &stubSalesforceEventService{}
	h := NewSalesforceEventHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/salesforce/events", strings.NewReader(
		`{"eventType":"CREATED","entity":"Account","referenceId":"001xx"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandleEvent(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if !svc.called || svc.req.ReferenceID != "001xx" {
		t.Errorf("service req = %+v", svc.req)
	}
}

func TestHandleEvent_ValidationError(t *testing.T) {
	svc := &stubSalesforceEventService{err: &apierror.ValidationError{Msg: "eventType UNDEFINED is not supported"}}
	h := NewSalesforceEventHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/salesforce/events", strings.NewReader(
		`{"eventType":"UNDEFINED","entity":"Account","referenceId":"001xx"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandleEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleEvent_UnknownFieldRejected(t *testing.T) {
	svc := &stubSalesforceEventService{}
	h := NewSalesforceEventHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/salesforce/events", strings.NewReader(
		`{"eventType":"CREATED","entity":"Account","referenceId":"001xx","extra":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandleEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if svc.called {
		t.Error("service must not be reached for an unknown field")
	}
}
