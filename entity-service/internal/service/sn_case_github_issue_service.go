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
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// validCaseGithubIssueReasons is the set of accepted CaseGithubIssueReason values.
var validCaseGithubIssueReasons = map[domain.CaseGithubIssueReason]struct{}{
	domain.CaseGithubIssueReasonDefault:   {},
	domain.CaseGithubIssueReasonMigration: {},
	domain.CaseGithubIssueReasonRDTicket:  {},
}

// snCaseGithubIssueRepoOverride mirrors CaseGithubIssueRepoOverride in the SN integration service payload.
type snCaseGithubIssueRepoOverride struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// snCaseGithubIssueCreatePayload mirrors POST /cases/{id}/github-issues in the SN integration service.
type snCaseGithubIssueCreatePayload struct {
	Reason         domain.CaseGithubIssueReason   `json:"reason"`
	Title          string                         `json:"title"`
	Description    string                         `json:"description"`
	RepoOverride   *snCaseGithubIssueRepoOverride `json:"repoOverride,omitempty"`
	UpdateLevel    *string                        `json:"updateLevel,omitempty"`
	PublicIssueURL *string                        `json:"publicIssueUrl,omitempty"`
	Regression     bool                           `json:"regression,omitempty"`
	HotFixRequired bool                           `json:"hotFixRequired,omitempty"`
	IssueTypeLabel *string                        `json:"issueTypeLabel,omitempty"`
	PriorityLevel  *string                        `json:"priorityLevel,omitempty"`
}

// snCaseGithubIssueCreateResponse mirrors the SN integration service POST /cases/{id}/github-issues response.
type snCaseGithubIssueCreateResponse struct {
	Message string `json:"message"`
	Issue   struct {
		URL    string `json:"url"`
		Number int    `json:"number"`
		Repo   string `json:"repo"`
	} `json:"issue"`
}

type snCaseGithubIssueService struct {
	client *integrationservice.Client
}

// NewServiceNowCaseGithubIssueService constructs a CaseGithubIssueService backed by the SN integration service.
func NewServiceNowCaseGithubIssueService(client *integrationservice.Client) CaseGithubIssueService {
	return &snCaseGithubIssueService{client: client}
}

// CreateCaseGithubIssue implements CaseGithubIssueService.
func (s *snCaseGithubIssueService) CreateCaseGithubIssue(ctx context.Context, req domain.CreateCaseGithubIssueRequest) (domain.CreateCaseGithubIssueResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	if req.CaseID == "" {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ValidationError{Msg: "case ID is required"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}
	if _, ok := validCaseGithubIssueReasons[req.Reason]; !ok {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ValidationError{Msg: "reason contains invalid value: " + string(req.Reason)}
	}
	if req.Title == "" {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ValidationError{Msg: "title is required"}
	}
	if req.Description == "" {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ValidationError{Msg: "description is required"}
	}
	if req.RepoOverride != nil && (req.RepoOverride.Owner == "" || req.RepoOverride.Repo == "") {
		return domain.CreateCaseGithubIssueResponse{}, &apierror.ValidationError{Msg: "repoOverride requires both owner and repo"}
	}

	payload := snCaseGithubIssueCreatePayload{
		Reason:         req.Reason,
		Title:          req.Title,
		Description:    req.Description,
		UpdateLevel:    req.UpdateLevel,
		PublicIssueURL: req.PublicIssueURL,
		Regression:     req.Regression,
		HotFixRequired: req.HotFixRequired,
		IssueTypeLabel: req.IssueTypeLabel,
		PriorityLevel:  req.PriorityLevel,
	}
	if req.RepoOverride != nil {
		payload.RepoOverride = &snCaseGithubIssueRepoOverride{
			Owner: req.RepoOverride.Owner,
			Repo:  req.RepoOverride.Repo,
		}
	}

	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(req.CaseID)+"/github-issues", token, payload)
	if err != nil {
		return domain.CreateCaseGithubIssueResponse{}, err
	}

	var snResp snCaseGithubIssueCreateResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateCaseGithubIssueResponse{}, fmt.Errorf("sn create case github issue: parse response: %w", err)
	}

	return domain.CreateCaseGithubIssueResponse{
		Message: snResp.Message,
		Issue: domain.CaseGithubIssueDetail{
			URL:    snResp.Issue.URL,
			Number: snResp.Issue.Number,
			Repo:   snResp.Issue.Repo,
		},
	}, nil
}
