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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A case created on the plain PostgreSQL data source starts with its
// description as the first comment, as ServiceNow's case-creation rules wrote
// it (and dual-write copied it): a COMMENT by the creator at the case's own
// creation time, written straight to the repository.

type descriptionCommentCall struct {
	req       domain.CreateCaseCommentRequest
	createdOn *time.Time
}

func descriptionCommentService(t *testing.T, calls *[]descriptionCommentCall, commentErr error) (*stubCaseRepo, CaseService) {
	t.Helper()
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1", CreatedBy: "jane.doe@example.com", CreatedOn: time.Date(2026, 11, 11, 6, 15, 38, 0, time.UTC)}, nil
		},
		createCaseCommentAsSystem: func(_ context.Context, req domain.CreateCaseCommentRequest, createdOn *time.Time) (domain.CaseComment, error) {
			*calls = append(*calls, descriptionCommentCall{req: req, createdOn: createdOn})
			return domain.CaseComment{}, commentErr
		},
	}
	return repo, NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
}

func TestCaseService_CreateCase_WritesTheDescriptionAsTheFirstComment(t *testing.T) {
	serviceRequest := validServiceRequestCreateCaseRequest()
	serviceRequest.Description = "Certificate Common Name: example.com"
	for name, req := range map[string]domain.CreateCaseRequest{
		"case":                     validCreateCaseRequest(),
		"service_request":          serviceRequest,
		"engagement":               validEngagementCreateCaseRequest(),
		"security_report_analysis": validSecurityReportAnalysisCreateCaseRequest(),
	} {
		t.Run(name, func(t *testing.T) {
			var calls []descriptionCommentCall
			_, svc := descriptionCommentService(t, &calls, nil)
			req.CreatedBy = "system"
			if _, err := svc.CreateCase(context.Background(), req); err != nil {
				t.Fatalf("CreateCase: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("comments written = %d, want 1 (the description)", len(calls))
			}
			got := calls[0]
			if got.req.CaseID != "case-1" || got.req.Type != domain.CommentTypeComment || got.req.Content != req.Description {
				t.Errorf("comment = %+v, want a COMMENT on case-1 holding the description %q", got.req, req.Description)
			}
			if got.req.CreatedBy != "jane.doe@example.com" {
				t.Errorf("createdBy = %q, want the case's creator", got.req.CreatedBy)
			}
			if got.createdOn == nil || !got.createdOn.Equal(time.Date(2026, 11, 11, 6, 15, 38, 0, time.UTC)) {
				t.Errorf("createdOn = %v, want the case's own creation time", got.createdOn)
			}
		})
	}
}

func TestCaseService_CreateCase_NoDescriptionCommentForAnAnnouncementOrABlankDescription(t *testing.T) {
	blank := validCreateCaseRequest()
	blank.Description = "   "
	for name, req := range map[string]domain.CreateCaseRequest{
		"announcement":      validAnnouncementCreateCaseRequest(),
		"blank description": blank,
	} {
		t.Run(name, func(t *testing.T) {
			var calls []descriptionCommentCall
			_, svc := descriptionCommentService(t, &calls, nil)
			req.CreatedBy = "system"
			_, _ = svc.CreateCase(context.Background(), req)
			if len(calls) != 0 {
				t.Errorf("comments written = %d, want none", len(calls))
			}
		})
	}
}

func TestCaseService_CreateCase_AFailedDescriptionCommentDoesNotFailTheCreate(t *testing.T) {
	var calls []descriptionCommentCall
	_, svc := descriptionCommentService(t, &calls, errors.New("database unavailable"))
	req := validCreateCaseRequest()
	req.CreatedBy = "system"
	resp, err := svc.CreateCase(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCase failed with the comment write: %v", err)
	}
	if resp.Case.ID != "case-1" || len(calls) != 1 {
		t.Errorf("resp = %+v, comment attempts = %d", resp, len(calls))
	}
}
