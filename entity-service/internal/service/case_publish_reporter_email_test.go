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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

func reporterEmailOf(t *testing.T, creator *domain.UserReference) string {
	t.Helper()
	caseType := "announcement"
	cv := domain.CaseView{
		ID:             "5e000000-0000-0000-0000-0000000d0044",
		Number:         "ANN-000001",
		Subject:        "Planned maintenance",
		Type:           &caseType,
		CreatedBy:      creator,
		ProjectDetails: &domain.EntityRef{ID: "p1", Name: "Project One"},
		WatchList:      []domain.WatchListUser{{ID: "u1", Email: "watcher@example.com"}},
	}
	pub := &mockEventPublisher{}
	getCase := func(context.Context, string) (domain.CaseView, error) { return cv, nil }
	publishCaseCreatedEvent(context.Background(), pub, getCase,
		func(context.Context, string, string) ([]string, error) { return []string{"contact@example.com"}, nil },
		nil, nil, domain.CreateCaseRequest{Type: "announcement", ProjectID: "p1"}, cv.ID)
	if len(pub.calls) != 1 || pub.calls[0].eventType != events.TypeCaseCreated {
		t.Fatalf("published %v, want one case.created", publishedTypes(pub.calls))
	}
	var p events.CaseCreatedPayload
	if err := json.Unmarshal(pub.calls[0].payload, &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return p.ReporterEmail
}

func TestPublishCaseCreatedEvent_CarriesReporterEmail(t *testing.T) {
	if got := reporterEmailOf(t, &domain.UserReference{Email: "jane.doe@example.com"}); got != "jane.doe@example.com" {
		t.Errorf("reporterEmail = %q, want jane.doe@example.com", got)
	}
}

func TestPublishCaseCreatedEvent_NilCreatorLeavesReporterEmailEmpty(t *testing.T) {
	if got := reporterEmailOf(t, nil); got != "" {
		t.Errorf("reporterEmail = %q, want empty", got)
	}
}
