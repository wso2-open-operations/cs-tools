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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type rawCapturingSync struct{ got service.Delivery }

func (r *rawCapturingSync) HandleWebhook(_ context.Context, d service.Delivery) (service.Outcome, error) {
	r.got = d
	return service.Outcome{Action: "x"}, nil
}
func (r *rawCapturingSync) CreateServiceRequestFromIssue(context.Context, domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error) {
	return domain.CreateServiceRequestFromIssueResponse{}, nil
}
func (r *rawCapturingSync) SetServiceRequestSeverityFromIssue(context.Context, string, string, int, domain.CaseSeverity) (domain.SetIssueSeverityResponse, error) {
	return domain.SetIssueSeverityResponse{}, nil
}

// The product issue branch reads fields IssuePayload does not carry, so the handler
// must hand the service the body it received, untouched.
func TestGithubDeliveryHandler_PassesTheRawPayload(t *testing.T) {
	svc := &rawCapturingSync{}
	h := NewGithubDeliveryHandler(svc, map[string]bool{"internal": true})
	body := `{"id":"d1","event":"projects_v2_item","payload":{"action":"edited","projects_v2_item":{"content_node_id":"I_x"}}}`
	req := httptest.NewRequest(http.MethodPost, "/github/deliveries", strings.NewReader(body))
	req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{ClientID: "internal", Validated: true}))
	rec := httptest.NewRecorder()
	h.Handle(rec, req)
	if rec.Code != http.StatusOK || svc.got.ID != "d1" || svc.got.Payload.Action != "edited" ||
		string(svc.got.Raw) != `{"action":"edited","projects_v2_item":{"content_node_id":"I_x"}}` {
		t.Fatalf("code=%d delivery=%+v", rec.Code, svc.got)
	}
}
