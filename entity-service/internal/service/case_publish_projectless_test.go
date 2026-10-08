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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// projectlessCase is a Postgres case with no project -- a service request
// raised from a GitHub issue -- that still has someone to email, so the
// publishers get past their no-recipients check.
func projectlessCase() domain.CaseView {
	return domain.CaseView{
		ID:        "5e000000-0000-0000-0000-0000000d0043",
		Number:    "SR-GH-000009",
		Subject:   "From GitHub",
		WatchList: []domain.WatchListUser{{ID: "u1", Email: "watcher@wso2.com"}},
	}
}

// A comment on a project-less case once panicked on cv.ProjectDetails.ID,
// failing the comment request with a 500 after the comment was committed.
func TestPublishCommentAddedEvent_ProjectlessCaseSkipsPublish(t *testing.T) {
	pub := &mockEventPublisher{}
	cv := projectlessCase()
	req := domain.CreateCaseCommentRequest{CaseID: cv.ID, Type: domain.CommentTypeComment, Content: "hi"}

	publishCommentAddedEvent(context.Background(), pub, nil, nil, cv, req, "c1", "Engineer", "engineer@wso2.com", true)

	if len(pub.calls) != 0 {
		t.Fatalf("published %v for a case with no project, want nothing", publishedTypes(pub.calls))
	}
}

func TestPublishStatusChangedEvent_ProjectlessCaseSkipsPublish(t *testing.T) {
	pub := &mockEventPublisher{}
	cv := projectlessCase()

	publishStatusChangedEvent(context.Background(), pub, nil, cv.ID, "Work In Progress", cv)

	if len(pub.calls) != 0 {
		t.Fatalf("published %v for a case with no project, want nothing", publishedTypes(pub.calls))
	}
}

func TestPublishCaseCreatedEvent_ProjectlessCaseSkipsPublish(t *testing.T) {
	pub := &mockEventPublisher{}
	cv := projectlessCase()
	getCase := func(context.Context, string) (domain.CaseView, error) { return cv, nil }

	publishCaseCreatedEvent(context.Background(), pub, getCase, nil, nil, domain.CreateCaseRequest{Type: "service_request"}, cv.ID)

	if len(pub.calls) != 0 {
		t.Fatalf("published %v for a case with no project, want nothing", publishedTypes(pub.calls))
	}
}
