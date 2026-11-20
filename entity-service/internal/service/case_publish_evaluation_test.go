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
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

const evalTestProjectID = "7a000000-0000-0000-0000-0000000e0001"

// evaluationCase is a case on a project with someone to email, so every
// publisher gets past its project and recipient checks.
func evaluationCase() domain.CaseView {
	sev := domain.CaseSeverityHigh
	return domain.CaseView{
		ID:             "7a000000-0000-0000-0000-0000000c0001",
		Number:         "CS0000001",
		Subject:        "Eval question",
		Severity:       &sev,
		CreatedOn:      time.Date(2026, 11, 20, 4, 0, 0, 0, time.UTC),
		ProjectDetails: &domain.EntityRef{ID: evalTestProjectID, Name: "Eval project"},
		WatchList:      []domain.WatchListUser{{ID: "u1", Email: "watcher@wso2.com"}},
	}
}

// onboardingInfo returns a resolver that answers isEvaluation for
// evalTestProjectID and records that it was asked.
func onboardingInfo(isEvaluation bool, err error, asked *[]string) projectOnboardingInfoResolver {
	return func(_ context.Context, projectID string) (string, bool, error) {
		*asked = append(*asked, projectID)
		if err != nil {
			return "", false, err
		}
		return "", isEvaluation && projectID == evalTestProjectID, nil
	}
}

func TestCaseIsEvaluationAccount(t *testing.T) {
	ctx := context.Background()
	var asked []string

	if !caseIsEvaluationAccount(ctx, onboardingInfo(true, nil, &asked), evaluationCase(), "test") {
		t.Error("evaluation project: got false, want true")
	}
	if caseIsEvaluationAccount(ctx, onboardingInfo(false, nil, &asked), evaluationCase(), "test") {
		t.Error("non-evaluation project: got true, want false")
	}
	// A failed lookup reads as false, so the card still goes to Incident
	// Monitor instead of the event being dropped.
	if caseIsEvaluationAccount(ctx, onboardingInfo(true, errors.New("db down"), &asked), evaluationCase(), "test") {
		t.Error("lookup error: got true, want false")
	}
	if caseIsEvaluationAccount(ctx, nil, evaluationCase(), "test") {
		t.Error("nil resolver: got true, want false")
	}

	asked = nil
	if caseIsEvaluationAccount(ctx, onboardingInfo(true, nil, &asked), projectlessCase(), "test") {
		t.Error("project-less case: got true, want false")
	}
	if len(asked) != 0 {
		t.Errorf("project-less case: resolver asked for %v, want no lookup", asked)
	}
}

func TestPublishCaseCreatedEvent_CarriesEvaluationFlag(t *testing.T) {
	for _, isEvaluation := range []bool{true, false} {
		pub := &mockEventPublisher{}
		cv := evaluationCase()
		getCase := func(context.Context, string) (domain.CaseView, error) { return cv, nil }
		var asked []string

		publishCaseCreatedEvent(context.Background(), pub, getCase, nil, nil, nil, onboardingInfo(isEvaluation, nil, &asked), domain.CreateCaseRequest{Type: "case"}, cv.ID)

		if len(pub.calls) != 1 || pub.calls[0].eventType != events.TypeCaseCreated {
			t.Fatalf("isEvaluation=%v: published %v, want one case.created", isEvaluation, publishedTypes(pub.calls))
		}
		var p events.CaseCreatedPayload
		if err := json.Unmarshal(pub.calls[0].payload, &p); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if p.IsEvaluationAccount != isEvaluation {
			t.Errorf("IsEvaluationAccount = %v, want %v", p.IsEvaluationAccount, isEvaluation)
		}
	}
}

func TestPublishSeverityChangedEvent_CarriesEvaluationFlag(t *testing.T) {
	for _, isEvaluation := range []bool{true, false} {
		pub := &mockEventPublisher{}
		var asked []string

		publishSeverityChangedEvent(context.Background(), pub, nil, nil, onboardingInfo(isEvaluation, nil, &asked), evaluationCase().ID, "high", "critical", evaluationCase())

		if len(pub.calls) != 1 || pub.calls[0].eventType != events.TypeSeverityChanged {
			t.Fatalf("isEvaluation=%v: published %v, want one case.severity_changed", isEvaluation, publishedTypes(pub.calls))
		}
		var p events.SeverityChangedPayload
		if err := json.Unmarshal(pub.calls[0].payload, &p); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if p.IsEvaluationAccount != isEvaluation {
			t.Errorf("IsEvaluationAccount = %v, want %v", p.IsEvaluationAccount, isEvaluation)
		}
	}
}
