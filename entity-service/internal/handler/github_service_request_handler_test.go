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

type fakeGithubSeveritySync struct {
	service.GithubSyncService
	owner, repository string
	issueNumber       int
	severity          domain.CaseSeverity
	calls             int
}

func (f *fakeGithubSeveritySync) SetServiceRequestSeverityFromIssue(_ context.Context, owner, repository string, issueNumber int, severity domain.CaseSeverity) (domain.SetIssueSeverityResponse, error) {
	f.calls++
	f.owner, f.repository, f.issueNumber, f.severity = owner, repository, issueNumber, severity
	return domain.SetIssueSeverityResponse{Message: "severity set", ID: "sr-1", Severity: severity}, nil
}

func severityRequest(clientID, issueNumber, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/github/service-requests/wso2/choreo/"+issueNumber+"/severity", strings.NewReader(body))
	req.SetPathValue("owner", "wso2")
	req.SetPathValue("repository", "choreo")
	req.SetPathValue("issueNumber", issueNumber)
	if clientID != "" {
		req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{ClientID: clientID, Validated: true}))
	}
	return req
}

func TestGithubServiceRequestHandler_SetSeverity(t *testing.T) {
	clients := map[string]bool{"workflow": true}
	t.Run("an internal client sets it", func(t *testing.T) {
		svc := &fakeGithubSeveritySync{}
		rec := httptest.NewRecorder()
		NewGithubServiceRequestHandler(svc, clients).SetSeverity(rec, severityRequest("workflow", "42", `{"severity":"critical"}`))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		if svc.owner != "wso2" || svc.repository != "choreo" || svc.issueNumber != 42 || svc.severity != domain.CaseSeverityCritical {
			t.Errorf("called with %s/%s#%d %q", svc.owner, svc.repository, svc.issueNumber, svc.severity)
		}
	})
	for name, tc := range map[string]struct {
		clientID, issueNumber string
		want                  int
	}{
		"no credential":       {"", "42", http.StatusUnauthorized},
		"an unlisted client":  {"someone-else", "42", http.StatusUnauthorized},
		"a non-numeric issue": {"workflow", "abc", http.StatusBadRequest},
		"issue zero":          {"workflow", "0", http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeGithubSeveritySync{}
			rec := httptest.NewRecorder()
			NewGithubServiceRequestHandler(svc, clients).SetSeverity(rec, severityRequest(tc.clientID, tc.issueNumber, `{"severity":"high"}`))
			if rec.Code != tc.want || svc.calls != 0 {
				t.Fatalf("status = %d, calls = %d, want %d and 0", rec.Code, svc.calls, tc.want)
			}
		})
	}
}
