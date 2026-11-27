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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// activeOnlyAlertMuteRuleService answers Active; the handler test reaches no other method.
type activeOnlyAlertMuteRuleService struct {
	service.AlertMuteRuleService
	activeCalls int
}

func (s *activeOnlyAlertMuteRuleService) Active(context.Context) (domain.ActiveAlertMuteRulesResponse, error) {
	s.activeCalls++
	return domain.ActiveAlertMuteRulesResponse{AlertMuteRules: []domain.AlertMuteRule{}}, nil
}

func TestListAlertMuteRules_ServesOnlyStatusActive(t *testing.T) {
	for target, want := range map[string]int{
		"/alert-mute-rules?status=active":  http.StatusOK,
		"/alert-mute-rules":                http.StatusBadRequest,
		"/alert-mute-rules?status=expired": http.StatusBadRequest,
		"/alert-mute-rules?status=Active":  http.StatusBadRequest,
	} {
		svc := &activeOnlyAlertMuteRuleService{}
		w := httptest.NewRecorder()
		NewAlertMuteRuleHandler(svc).ListAlertMuteRules(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != want {
			t.Errorf("%s: status %d, want %d", target, w.Code, want)
		}
		if wantCalls := map[bool]int{true: 1, false: 0}[want == http.StatusOK]; svc.activeCalls != wantCalls {
			t.Errorf("%s: service called %d times, want %d", target, svc.activeCalls, wantCalls)
		}
	}
}
