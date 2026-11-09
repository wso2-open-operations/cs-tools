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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type recordingSeveritySetter struct {
	id       string
	severity domain.CaseSeverity
	calls    int
}

func (r *recordingSeveritySetter) SetServiceRequestSeverity(_ context.Context, id string, severity domain.CaseSeverity) (domain.UpdateCaseResponse, error) {
	r.calls++
	r.id, r.severity = id, severity
	return domain.UpdateCaseResponse{Case: domain.UpdatedCase{ID: id, Severity: &severity}}, nil
}

func severitySvc(r *fakeGhRepo, setter *recordingSeveritySetter) GithubSyncService {
	svc := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{})
	svc.(*githubSyncService).severities = setter
	return svc
}

func TestGithubTemplateSeverity(t *testing.T) {
	cases := map[string]domain.CaseSeverity{
		"Critical":                domain.CaseSeverityCritical,
		"Critical (System Down)":  domain.CaseSeverityCritical,
		"2 - High":                domain.CaseSeverityHigh,
		"Moderate":                domain.CaseSeverityMedium,
		"moderate (Minor Impact)": domain.CaseSeverityMedium,
		"Low":                     domain.CaseSeverityLow,
		"":                        "",
		"Whenever":                "",
	}
	for in, want := range cases {
		if got := githubTemplateSeverity(in); got != want {
			t.Errorf("githubTemplateSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandleWebhook_TemplatePrioritySetsTheInitialSeverity(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped()}
	m := &fakeGhMutations{}
	d := ghDelivery("issues", "labeled", "[CR]: rotate certs", []string{"CR/NormalChange"})
	d.Payload.Issue.Body = "### Priority\n\nHigh"
	if _, err := writingSvc(r, m, &fakeGhClient{}).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if m.lastSR.Severity != domain.CaseSeverityHigh {
		t.Errorf("severity = %q, want high", m.lastSR.Severity)
	}
}

func TestSetServiceRequestSeverityFromIssue_SetsTheIssuesRecord(t *testing.T) {
	setter := &recordingSeveritySetter{}
	out, err := severitySvc(&fakeGhRepo{mapping: mapped(), caseID: "sr-1"}, setter).
		SetServiceRequestSeverityFromIssue(context.Background(), "wso2", "choreo", 42, domain.CaseSeverityCritical)
	if err != nil {
		t.Fatalf("SetServiceRequestSeverityFromIssue: %v", err)
	}
	if setter.id != "sr-1" || setter.severity != domain.CaseSeverityCritical {
		t.Errorf("set %q to %q, want sr-1 to critical", setter.id, setter.severity)
	}
	if out.ID != "sr-1" || out.Severity != domain.CaseSeverityCritical {
		t.Errorf("response = %+v", out)
	}
}

func TestSetServiceRequestSeverityFromIssue_Refusals(t *testing.T) {
	var validation *apierror.ValidationError
	var conflict *apierror.ConflictError
	var notFound *apierror.NotFoundError
	cases := []struct {
		name     string
		repo     *fakeGhRepo
		severity domain.CaseSeverity
		want     any
	}{
		{"catastrophic is not a service request severity", &fakeGhRepo{mapping: mapped(), caseID: "sr-1"}, domain.CaseSeverityCatastrophic, &validation},
		{"unknown severity", &fakeGhRepo{mapping: mapped(), caseID: "sr-1"}, "urgent", &validation},
		{"unmapped repository", &fakeGhRepo{}, domain.CaseSeverityHigh, &conflict},
		{"issue without a service request", &fakeGhRepo{mapping: mapped()}, domain.CaseSeverityHigh, &notFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setter := &recordingSeveritySetter{}
			_, err := severitySvc(tc.repo, setter).
				SetServiceRequestSeverityFromIssue(context.Background(), "wso2", "choreo", 42, tc.severity)
			if !errors.As(err, tc.want) {
				t.Fatalf("err = %v, want %T", err, tc.want)
			}
			if setter.calls != 0 {
				t.Errorf("severity was written %d times, want none", setter.calls)
			}
		})
	}
}

func TestSetServiceRequestSeverityFromIssue_UnavailableWithoutThePostgresCaseService(t *testing.T) {
	svc := writingSvc(&fakeGhRepo{mapping: mapped(), caseID: "sr-1"}, &fakeGhMutations{}, &fakeGhClient{})
	_, err := svc.SetServiceRequestSeverityFromIssue(context.Background(), "wso2", "choreo", 42, domain.CaseSeverityHigh)
	var e *apierror.ServiceUnavailableError
	if !errors.As(err, &e) {
		t.Fatalf("err = %v, want service unavailable", err)
	}
}
