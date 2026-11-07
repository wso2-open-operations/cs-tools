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

package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
)

type stubLongOpenSvc struct {
	got   domain.LongOpenCaseReportRequest
	calls int
}

func (s *stubLongOpenSvc) BuildReport(_ context.Context, req domain.LongOpenCaseReportRequest) (domain.LongOpenCaseReport, error) {
	s.calls++
	s.got = req
	return domain.LongOpenCaseReport{Cases: []domain.LongOpenCaseReportRow{}, OnboardingCases: []domain.LongOpenCaseReportRow{}}, nil
}

func getLongOpen(t *testing.T, query string) (*stubLongOpenSvc, *httptest.ResponseRecorder) {
	t.Helper()
	stub := &stubLongOpenSvc{}
	h := handler.NewLongOpenCaseReportHandler(stub)
	rec := httptest.NewRecorder()
	h.GetLongOpenCaseReport(rec, httptest.NewRequest(http.MethodGet, "/reports/long-open-cases"+query, nil))
	return stub, rec
}

func TestLongOpenCaseReportHandlerParams(t *testing.T) {
	stub, rec := getLongOpen(t, "")
	if rec.Code != 200 || stub.got.Variant != "" || stub.got.OlderThanDays != 0 {
		t.Errorf("default: code %d req %+v", rec.Code, stub.got)
	}
	// The default response carries neither new field.
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["variant"]; ok {
		t.Errorf("default response must not carry variant: %s", rec.Body.String())
	}

	stub, rec = getLongOpen(t, "?olderThanDays=7")
	if rec.Code != 200 || stub.got.OlderThanDays != 7 {
		t.Errorf("olderThanDays: code %d req %+v", rec.Code, stub.got)
	}

	stub, rec = getLongOpen(t, "?variant=open-before-yesterday")
	if rec.Code != 200 || stub.got.Variant != domain.LongOpenCaseReportVariantOpenBeforeYesterday {
		t.Errorf("variant: code %d req %+v", rec.Code, stub.got)
	}
}

func TestLongOpenCaseReportHandlerRejectsBadVariant(t *testing.T) {
	for _, q := range []string{"?variant=nope", "?variant=open-before-yesterday&olderThanDays=3", "?olderThanDays=0"} {
		stub, rec := getLongOpen(t, q)
		if rec.Code != http.StatusBadRequest || stub.calls != 0 {
			t.Errorf("%s: code %d calls %d, want 400 and no service call (%s)", q, rec.Code, stub.calls, strings.TrimSpace(rec.Body.String()))
		}
	}
}
