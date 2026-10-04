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
)

// TestPublishHelpers_CaseWithoutProject proves the shared publish helpers
// accept a case with no project linked (ProjectDetails nil, a valid state on
// the Postgres data source) and publish with an empty project id instead of
// panicking after the write has already committed.
func TestPublishHelpers_CaseWithoutProject(t *testing.T) {
	cv := domain.CaseView{
		ID: authzCaseID, Number: "CS0001", Subject: "s",
		WatchList: []domain.WatchListUser{{Email: "watcher@example.com"}},
	}
	noDefaults := func(context.Context, string) ([]string, error) { return nil, nil }
	ctx := context.Background()

	pub := &mockEventPublisher{}
	publishCaseCreatedEvent(ctx, pub,
		func(context.Context, string) (domain.CaseView, error) { return cv, nil },
		func(context.Context, string, string) ([]string, error) { return nil, nil },
		noDefaults, domain.CreateCaseRequest{}, authzCaseID)
	publishCommentAddedEvent(ctx, pub, noDefaults, cv, domain.CreateCaseCommentRequest{CaseID: authzCaseID, Type: domain.CommentTypeComment, Content: "x"}, "c1", "Jane Doe")
	publishStatusChangedEvent(ctx, pub, noDefaults, authzCaseID, "Closed", cv)
	(&snCaseService{publisher: pub}).publishCaseAssigned(ctx, authzCaseID, "Jane Doe", "jane.doe@example.com", cv)

	if len(pub.calls) == 0 {
		t.Fatal("expected at least one publish for a case with a watcher")
	}
	for _, c := range pub.calls {
		var payload struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.Unmarshal(c.payload, &payload); err != nil {
			t.Fatalf("%s: payload: %v", c.eventType, err)
		}
		if payload.ProjectID != "" {
			t.Errorf("%s: projectId = %q, want empty for a case with no project", c.eventType, payload.ProjectID)
		}
	}
}
