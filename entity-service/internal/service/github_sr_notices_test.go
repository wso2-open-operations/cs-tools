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

// recordingSRNotifier is sr_notice_service_test.go's.

// writingSvcWithNotices is writingSvc with the SR automation attached.
func writingSvcWithNotices(m *fakeGhMutations, n *recordingSRNotifier) GithubSyncService {
	svc := writingSvc(&fakeGhRepo{mapping: mapped()}, m, &fakeGhClient{})
	svc.(*githubSyncService).srNotices = n
	return svc
}

// An SR created from a GitHub issue gets the same automation as one created
// in the portal -- assignment, acknowledgement and the sr.created Chat card --
// whichever way it arrives: a webhook delivery or the internal create call.
func TestGithubSR_NewRecordRunsTheSRAutomation(t *testing.T) {
	n := &recordingSRNotifier{}
	if _, err := writingSvcWithNotices(&fakeGhMutations{}, n).HandleWebhook(context.Background(),
		ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates", []string{"CR/NormalChange"})); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(n.created) != 1 || n.created[0] != "sr-new" {
		t.Errorf("webhook: automation ran for %v, want [sr-new]", n.created)
	}

	n = &recordingSRNotifier{}
	if _, err := writingSvcWithNotices(&fakeGhMutations{}, n).CreateServiceRequestFromIssue(context.Background(),
		domain.CreateServiceRequestFromIssueRequest{
			Owner: "acme", Repository: "widgets", IssueNumber: 42,
			Title: "[CR]: something", Labels: []string{"CR/NormalChange", "validation-passed"},
		}); err != nil {
		t.Fatalf("CreateServiceRequestFromIssue: %v", err)
	}
	if len(n.created) != 1 || n.created[0] != "sr-new" {
		t.Errorf("create call: automation ran for %v, want [sr-new]", n.created)
	}
}

// No second announcement: a delivery that lost the insert race found the
// record another delivery created, and that one announced it.
func TestGithubSR_ExistingRecordIsNotAnnouncedAgain(t *testing.T) {
	n := &recordingSRNotifier{}
	if _, err := writingSvcWithNotices(&fakeGhMutations{srAlreadyExists: true}, n).HandleWebhook(context.Background(),
		ghDelivery("issues", "labeled", "[CR]: rotate gateway certificates", []string{"CR/NormalChange"})); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(n.created) != 0 {
		t.Errorf("automation ran for %v, want none for an existing record", n.created)
	}
}

// Nothing created, nothing announced: an issue the validation workflow has not
// passed yet.
func TestGithubSR_UnvalidatedIssueIsNotAnnounced(t *testing.T) {
	d := ghDelivery("issues", "opened", "[CR]: rotate gateway certificates", []string{"CR/NormalChange"})
	d.Payload.Issue.Labels = d.Payload.Issue.Labels[:1]
	n := &recordingSRNotifier{}
	if _, err := writingSvcWithNotices(&fakeGhMutations{}, n).HandleWebhook(context.Background(), d); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(n.created) != 0 {
		t.Errorf("automation ran for %v, want none", n.created)
	}
}

// WithGithubSRNotices ignores a nil service, so a deployment without
// SRE_EVENT_HUB_TOPIC runs no automation rather than a nil one.
func TestWithGithubSRNotices_NilIsANoOp(t *testing.T) {
	svc := WithGithubSRNotices(writingSvc(&fakeGhRepo{mapping: mapped()}, &fakeGhMutations{}, &fakeGhClient{}), nil)
	if svc.(*githubSyncService).srNotices != nil {
		t.Error("a nil SR notice service must leave the sync without automation")
	}
}
