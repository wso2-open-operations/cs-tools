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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// githubTemplateSeverities maps the issue template's Priority dropdown to a service request severity.
var githubTemplateSeverities = map[string]domain.CaseSeverity{
	"critical": domain.CaseSeverityCritical,
	"high":     domain.CaseSeverityHigh,
	"moderate": domain.CaseSeverityMedium,
	"low":      domain.CaseSeverityLow,
}

// githubTemplateSeverity reads "Critical", "1 - Critical" or "Critical (System Down)"; anything else is "" (the default).
func githubTemplateSeverity(raw string) domain.CaseSeverity {
	v := strings.TrimSpace(snParenthesisedRe.ReplaceAllString(raw, ""))
	if _, rest, ok := strings.Cut(v, " - "); ok {
		v = rest
	}
	return githubTemplateSeverities[strings.ToLower(strings.TrimSpace(v))]
}

// SetServiceRequestSeverityFromIssue implements GithubSyncService.
func (s *githubSyncService) SetServiceRequestSeverityFromIssue(ctx context.Context, owner, repository string, issueNumber int, severity domain.CaseSeverity) (domain.SetIssueSeverityResponse, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repository) == "" {
		return domain.SetIssueSeverityResponse{}, &apierror.ValidationError{Msg: "owner and repository are required"}
	}
	if issueNumber <= 0 {
		return domain.SetIssueSeverityResponse{}, &apierror.ValidationError{Msg: "issueNumber must be a positive integer"}
	}
	if !validServiceRequestSeverity[severity] {
		return domain.SetIssueSeverityResponse{}, &apierror.ValidationError{Msg: "severity must be one of critical, high, medium, low"}
	}
	if s.severities == nil {
		return domain.SetIssueSeverityResponse{}, &apierror.ServiceUnavailableError{Msg: "service request severity is not available for this data source"}
	}

	mapping, reason, err := s.resolveMapping(ctx, owner, repository)
	if err != nil {
		return domain.SetIssueSeverityResponse{}, err
	}
	if reason != "" {
		return domain.SetIssueSeverityResponse{}, &apierror.ConflictError{Msg: reason}
	}
	id, err := s.repo.CaseByIssue(ctx, mapping.AccountID, owner, repository, issueNumber)
	if err != nil {
		return domain.SetIssueSeverityResponse{}, err
	}
	if id == "" {
		return domain.SetIssueSeverityResponse{}, &apierror.NotFoundError{Msg: "no service request for this issue"}
	}

	resp, err := s.severities.SetServiceRequestSeverity(ctx, id, severity)
	if err != nil {
		return domain.SetIssueSeverityResponse{}, err
	}
	out := domain.SetIssueSeverityResponse{Message: "severity set", ID: id, Severity: severity}
	if resp.Case.Severity != nil {
		out.Severity = *resp.Case.Severity
	}
	return out, nil
}
