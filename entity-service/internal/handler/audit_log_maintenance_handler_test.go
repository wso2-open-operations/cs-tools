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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeAuditLogMaintenanceService struct{ calls int }

func (f *fakeAuditLogMaintenanceService) RunMaintenance(context.Context) (domain.AuditLogMaintenanceResponse, error) {
	f.calls++
	return domain.AuditLogMaintenanceResponse{}, nil
}

func TestAuditLogMaintenanceHandler_Body(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCode  int
		wantCalls int
	}{
		{"no body", "", http.StatusOK, 1},
		{"empty object", "{}", http.StatusOK, 1},
		{"malformed json", "{not json", http.StatusBadRequest, 0},
		{"unknown field", `{"monthsAhead": 12}`, http.StatusBadRequest, 0},
		{"two objects", "{}{}", http.StatusBadRequest, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeAuditLogMaintenanceService{}
			h := NewAuditLogMaintenanceHandler(svc)

			req := httptest.NewRequest(http.MethodPost, "/audit-log/maintenance", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.Run(rec, req)

			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if svc.calls != tc.wantCalls {
				t.Errorf("RunMaintenance called %d times, want %d", svc.calls, tc.wantCalls)
			}
		})
	}
}
