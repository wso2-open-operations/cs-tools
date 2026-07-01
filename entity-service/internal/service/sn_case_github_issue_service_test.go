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

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// contextWithUserIDToken builds a context carrying the given x-user-id-token,
// going through the real middleware so the private context key stays
// encapsulated in the middleware package.
func contextWithUserIDToken(token string) context.Context {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	var captured context.Context
	middleware.UserIDToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), req)
	return captured
}

func TestSNCaseGithubIssueService_CreateCaseGithubIssue_Validation(t *testing.T) {
	validCaseID := "11111111-1111-1111-1111-111111111111"

	tests := []struct {
		name    string
		ctx     context.Context
		req     domain.CreateCaseGithubIssueRequest
		wantErr any // pointer type to assert via errors.As-style type switch below
	}{
		{
			name: "missing token",
			ctx:  context.Background(),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "d",
			},
			wantErr: &apierror.UnauthorizedError{},
		},
		{
			name: "empty case ID",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      "",
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "d",
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "invalid case ID format",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      "not-a-uuid",
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "d",
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "invalid reason",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      "not_a_reason",
				Title:       "t",
				Description: "d",
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "missing title",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "",
				Description: "d",
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "missing description",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "",
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "repoOverride missing repo",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "d",
				RepoOverride: &domain.CaseGithubIssueRepoOverride{
					Owner: "wso2-enterprise",
					Repo:  "",
				},
			},
			wantErr: &apierror.ValidationError{},
		},
		{
			name: "repoOverride missing owner",
			ctx:  contextWithUserIDToken("token"),
			req: domain.CreateCaseGithubIssueRequest{
				CaseID:      validCaseID,
				Reason:      domain.CaseGithubIssueReasonDefault,
				Title:       "t",
				Description: "d",
				RepoOverride: &domain.CaseGithubIssueRepoOverride{
					Owner: "",
					Repo:  "wso2-apim-internal",
				},
			},
			wantErr: &apierror.ValidationError{},
		},
	}

	// client is intentionally nil: every case above must fail validation
	// before the service ever touches s.client.Post.
	svc := NewServiceNowCaseGithubIssueService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCaseGithubIssue(tt.ctx, tt.req)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			switch tt.wantErr.(type) {
			case *apierror.UnauthorizedError:
				if _, ok := err.(*apierror.UnauthorizedError); !ok {
					t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
				}
			case *apierror.ValidationError:
				if _, ok := err.(*apierror.ValidationError); !ok {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
			}
		})
	}
}
