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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeProductIssueSyncService struct {
	reconcileReq domain.ProductIssueReconcileRequest
	backfillReq  domain.ProductIssueBackfillRequest
	err          error
}

func (f *fakeProductIssueSyncService) Reconcile(_ context.Context, req domain.ProductIssueReconcileRequest) (domain.ProductIssueReconcileResponse, error) {
	f.reconcileReq = req
	return domain.ProductIssueReconcileResponse{Boards: 1, More: true, NextCursor: "c", RetryAfterSeconds: 30}, f.err
}

func (f *fakeProductIssueSyncService) Backfill(_ context.Context, req domain.ProductIssueBackfillRequest) (domain.ProductIssueBackfillResponse, error) {
	f.backfillReq = req
	return domain.ProductIssueBackfillResponse{DryRun: req.DryRun, FailedIssueNumbers: []int{}}, f.err
}

func TestProductIssueSyncHandler_Reconcile(t *testing.T) {
	svc := &fakeProductIssueSyncService{}
	h := NewProductIssueSyncHandler(svc)

	// An empty body starts a run.
	w := httptest.NewRecorder()
	h.Reconcile(w, httptest.NewRequest(http.MethodPost, "/product-work-items/reconcile", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"retryAfterSeconds":30`) || !strings.Contains(w.Body.String(), `"nextCursor":"c"`) {
		t.Fatalf("empty body: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.Reconcile(w, httptest.NewRequest(http.MethodPost, "/product-work-items/reconcile", strings.NewReader(`{"cursor":"abc","limit":5}`)))
	if w.Code != http.StatusOK || svc.reconcileReq.Cursor != "abc" || svc.reconcileReq.Limit != 5 {
		t.Fatalf("with a body: %d %+v", w.Code, svc.reconcileReq)
	}

	w = httptest.NewRecorder()
	h.Reconcile(w, httptest.NewRequest(http.MethodPost, "/product-work-items/reconcile", strings.NewReader(`{"bogus":1}`)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("an unknown field: %d", w.Code)
	}

	svc.err = &apierror.ValidationError{Msg: "cursor is not valid."}
	w = httptest.NewRecorder()
	h.Reconcile(w, httptest.NewRequest(http.MethodPost, "/product-work-items/reconcile", strings.NewReader(`{"cursor":"x"}`)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "cursor is not valid") {
		t.Errorf("a validation error: %d %s", w.Code, w.Body.String())
	}
}

func TestProductIssueSyncHandler_Backfill(t *testing.T) {
	svc := &fakeProductIssueSyncService{}
	h := NewProductIssueSyncHandler(svc)

	w := httptest.NewRecorder()
	h.Backfill(w, httptest.NewRequest(http.MethodPost, "/product-work-items/backfill",
		strings.NewReader(`{"owner":"example-org","repository":"example-repo","dryRun":true,"maxIssues":10,"sinceDays":30,"cursor":"c"}`)))
	if w.Code != http.StatusOK || !svc.backfillReq.DryRun || svc.backfillReq.MaxIssues != 10 || svc.backfillReq.SinceDays != 30 ||
		svc.backfillReq.Owner != "example-org" || svc.backfillReq.Cursor != "c" || !strings.Contains(w.Body.String(), `"dryRun":true`) {
		t.Fatalf("backfill: %d %+v %s", w.Code, svc.backfillReq, w.Body.String())
	}

	// The body is required, and unknown fields are refused.
	for _, body := range []string{``, `{"owner":"o","repository":"r","nope":true}`} {
		w = httptest.NewRecorder()
		h.Backfill(w, httptest.NewRequest(http.MethodPost, "/product-work-items/backfill", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q: %d", body, w.Code)
		}
	}
}
