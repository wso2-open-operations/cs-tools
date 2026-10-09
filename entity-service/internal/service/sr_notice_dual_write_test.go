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
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// TestSRNoticeService_OnCreatedMirrored: the acknowledgement comment is
// handed to the mirror once it is written here -- and only then.
func TestSRNoticeService_OnCreatedMirrored(t *testing.T) {
	tests := []struct {
		name       string
		alertTeams []string
		ackErr     error
		wantMirror bool
	}{
		{"automated team: acknowledged and mirrored", []string{srTestTeamID}, nil, true},
		{"team not automated: nothing to mirror", nil, nil, false},
		{"acknowledgement failed: nothing to mirror", []string{srTestTeamID}, errors.New("db down"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true, ackErr: tt.ackErr}
			pub := &mockEventPublisher{}
			var mirrored []string
			NewSRNoticeService(repo, pub, tt.alertTeams).OnCreatedMirrored(context.Background(), srTestCaseID,
				func(_ context.Context, caseID, comment string) {
					if caseID != srTestCaseID {
						t.Errorf("mirrored case %q, want %q", caseID, srTestCaseID)
					}
					mirrored = append(mirrored, comment)
				})

			if !tt.wantMirror {
				if len(mirrored) != 0 {
					t.Errorf("mirrored %q, want nothing", mirrored)
				}
				return
			}
			if len(mirrored) != 1 || mirrored[0] != srAcknowledgement {
				t.Fatalf("mirrored %q, want the acknowledgement once", mirrored)
			}
			if got := srPublishedTypes(pub); len(got) != 2 || got[0] != events.TypeSRCreated || got[1] != events.TypeSRAcknowledged {
				t.Errorf("published %v, want sr.created then sr.acknowledged", got)
			}
		})
	}
}

// bareCommentMirror stands in for snCaseService on the dual-write mirror: it
// records the bare comments written to ServiceNow.
type bareCommentMirror struct {
	CaseService
	mu       sync.Mutex
	comments []string
}

func (m *bareCommentMirror) CreateBareCaseComment(_ context.Context, caseID string, commentType domain.CommentType, content string) (domain.CaseCommentDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if commentType != domain.CommentTypeComment {
		return domain.CaseCommentDetail{}, errors.New("want a customer-visible comment")
	}
	m.comments = append(m.comments, caseID+"|"+content)
	return domain.CaseCommentDetail{ID: "sn-comment-1"}, nil
}

func (m *bareCommentMirror) written() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.comments...)
}

// TestCaseService_NotifySRCreatedOnSNFirst: an SR created ServiceNow-first
// gets the SR automation only when SR_CREATION_NOTICES_ENABLED leaves it on,
// and its acknowledgement then reaches ServiceNow through the writeback.
func TestCaseService_NotifySRCreatedOnSNFirst(t *testing.T) {
	t.Run("switched off: left to ServiceNow's flow", func(t *testing.T) {
		n := &recordingSRNotifier{}
		svc := &caseService{srNotices: n}
		svc.notifySRCreatedOnSNFirst(context.Background(), srTestCaseID)
		if len(n.created) != 0 {
			t.Errorf("OnCreated called %v, want not at all", n.created)
		}
	})

	t.Run("switched on: runs the automation", func(t *testing.T) {
		n := &recordingSRNotifier{}
		svc := WithSRCreationNoticesOnDualWrite(&caseService{srNotices: n}).(*caseService)
		svc.notifySRCreatedOnSNFirst(context.Background(), srTestCaseID)
		if len(n.created) != 1 || n.created[0] != srTestCaseID {
			t.Errorf("OnCreated called %v, want once for %s", n.created, srTestCaseID)
		}
	})

	t.Run("switched on: acknowledgement mirrored to ServiceNow", func(t *testing.T) {
		repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true}
		mirror := &bareCommentMirror{}
		svc := WithSRCreationNoticesOnDualWrite(&caseService{
			srNotices:   NewSRNoticeService(repo, &mockEventPublisher{}, []string{srTestTeamID}),
			snMirror:    mirror,
			snWriteback: NewSNWritebackDispatcher(&recordingSNWritebackFailures{}),
		}).(*caseService)

		svc.notifySRCreatedOnSNFirst(context.Background(), srTestCaseID)

		if len(repo.assigned) != 1 || len(repo.acknowledged) != 1 {
			t.Fatalf("assigned %v, acknowledged %v; want one each", repo.assigned, repo.acknowledged)
		}
		// The writeback runs on its own worker pool.
		want := srTestCaseID + "|" + srAcknowledgement
		deadline := time.Now().Add(5 * time.Second)
		for {
			if got := mirror.written(); len(got) == 1 && got[0] == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("ServiceNow comments %q, want [%q]", mirror.written(), want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
}
