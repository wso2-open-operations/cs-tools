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

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

const srTestTeam = "MS/PC SRE Group"

// srRecord builds an sr.* record for SR-1 / SR0001001; team is the payload's
// sreTeamName and extra is appended to the payload's fields.
func srRecord(typ, team, extra string) eventbus.Record {
	payload := `"caseId":"SR-1","number":"SR0001001","wso2CaseId":"WSO2-77","subject":"Open port 443","sreTeamId":"T-1","sreTeamName":"` + team + `","assignmentGroupName":"MS/PC SRE Group"`
	if extra != "" {
		payload += "," + extra
	}
	return eventbus.Record{Topic: "sre-events", Value: []byte(`{"type":"` + typ + `","entityId":"SR-1","payload":{` + payload + `}}`)}
}

const (
	srCreatedExtra = `"description":"<p>Please open <b>443</b></p>","state":"Open","createdOn":"2026-10-07T10:00:00Z"`
	srAckExtra     = `"commentId":"C-ACK"`
)

func srCommentExtra(commentType, tags string) string {
	return `"commentId":"C-2","commentType":"` + commentType + `","content":"Any update?","authorEmail":"jane@acme.com","authorName":"Jane Doe","tags":` + tags + `,"createdOn":"2026-10-07T10:05:00Z"`
}

func newSRTestDispatcher(chat *mockGoogleChatSender, links *mockLinkResolver) *Dispatcher {
	if chat.hasAudienceSpace == nil {
		chat.hasAudienceSpace = func(a string) bool { return a == srTestTeam }
	}
	return NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, links, true, false, nil, true, "", nil)
}

// TestDispatcher_HandleShared_SR_PostsToSRETeam: each sr.* type, arriving on
// sre-events through HandleShared, posts its card to the SR's SRE team
// audience with the SR's own number (which keys the shared thread) and the
// CSM portal link.
func TestDispatcher_HandleShared_SR_PostsToSRETeam(t *testing.T) {
	cases := []struct {
		name   string
		record eventbus.Record
		check  func(t *testing.T, got sentSRAlert)
	}{
		{"sr.created", srRecord("sr.created", srTestTeam, srCreatedExtra), func(t *testing.T, got sentSRAlert) {
			a := got.created
			if got.kind != "created" || a.Number != "SR0001001" || a.WSO2CaseID != "WSO2-77" || a.Subject != "Open port 443" ||
				a.AssignmentGroupName != "MS/PC SRE Group" || a.State != "Open" || a.Description != "<p>Please open <b>443</b></p>" ||
				a.CaseLink != "https://csm.example/operations/service-requests/SR-1" {
				t.Errorf("unexpected created alert: %+v", got)
			}
		}},
		{"sr.acknowledged", srRecord("sr.acknowledged", srTestTeam, srAckExtra), func(t *testing.T, got sentSRAlert) {
			a := got.acked
			if got.kind != "acknowledged" || a.Number != "SR0001001" || a.WSO2CaseID != "WSO2-77" ||
				a.AssignmentGroupName != "MS/PC SRE Group" || a.SRETeamName != srTestTeam || a.CaseLink != "https://csm.example/operations/service-requests/SR-1" {
				t.Errorf("unexpected acknowledged alert: %+v", got)
			}
		}},
		{"sr.comment_added", srRecord("sr.comment_added", srTestTeam, srCommentExtra("comment", `["devops-sm"]`)), func(t *testing.T, got sentSRAlert) {
			a := got.comment
			if got.kind != "comment" || a.Number != "SR0001001" || a.WSO2CaseID != "WSO2-77" || a.Subject != "Open port 443" ||
				a.AuthorName != "Jane Doe" || a.AuthorEmail != "jane@acme.com" || a.Content != "Any update?" ||
				a.CommentLink != "https://csm.example/operations/service-requests/SR-1#C-2" {
				t.Errorf("unexpected comment alert: %+v", got)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chat := &mockGoogleChatSender{}
			d := newSRTestDispatcher(chat, &mockLinkResolver{isCustomer: true})
			if err := d.HandleShared(context.Background(), c.record); err != nil {
				t.Fatalf("HandleShared() error = %v", err)
			}
			if len(chat.srCalls) != 1 {
				t.Fatalf("SR chat sends = %d, want 1", len(chat.srCalls))
			}
			if chat.srCalls[0].audience != srTestTeam {
				t.Errorf("audience = %q, want %q", chat.srCalls[0].audience, srTestTeam)
			}
			c.check(t, chat.srCalls[0])
			if len(d.done) != 0 {
				t.Errorf("d.done = %v, want the chat claim released after a successful post", d.done)
			}
		})
	}
}

// TestDispatcher_Handle_SR_NoSpaceIsANoOp: an SR with no SRE team, or whose
// team has no configured Chat space, posts nothing and is not an error -- a
// retry could not fix either.
func TestDispatcher_Handle_SR_NoSpaceIsANoOp(t *testing.T) {
	types := map[string]string{
		"sr.created":       srCreatedExtra,
		"sr.acknowledged":  srAckExtra,
		"sr.comment_added": srCommentExtra("comment", `["devops-sm"]`),
	}
	for typ, extra := range types {
		for _, team := range []string{"", "  ", "Unconfigured SRE Group"} {
			t.Run(fmt.Sprintf("%s team=%q", typ, team), func(t *testing.T) {
				chat := &mockGoogleChatSender{}
				d := newSRTestDispatcher(chat, &mockLinkResolver{isCustomer: true})
				if err := d.Handle(context.Background(), srRecord(typ, team, extra)); err != nil {
					t.Fatalf("Handle() error = %v, want nil", err)
				}
				if chat.srStarted.Load() != 0 {
					t.Errorf("SR chat sends = %d, want 0", chat.srStarted.Load())
				}
			})
		}
	}
}

// TestDispatcher_Handle_SRCommentAdded_OnlyCustomerCommentsOnDevopsSM: the
// customer-comment alert needs all three of a customer-visible comment, the
// devops-sm tag (trimmed, any case) and a customer author.
func TestDispatcher_Handle_SRCommentAdded_OnlyCustomerCommentsOnDevopsSM(t *testing.T) {
	cases := []struct {
		name        string
		commentType string
		tags        string
		isCustomer  bool
		wantSends   int
	}{
		{"customer comment, devops-sm", "comment", `["devops-sm"]`, true, 1},
		{"tag case and whitespace ignored", "comment", `["  DevOps-SM "]`, true, 1},
		{"tag among others", "comment", `["network","devops-sm","urgent"]`, true, 1},
		{"work note", "work_note", `["devops-sm"]`, true, 0},
		{"non-customer author", "comment", `["devops-sm"]`, false, 0},
		{"no tags", "comment", `[]`, true, 0},
		{"null tags", "comment", `null`, true, 0},
		{"other tag only", "comment", `["devops"]`, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chat := &mockGoogleChatSender{}
			d := newSRTestDispatcher(chat, &mockLinkResolver{isCustomer: c.isCustomer})
			if err := d.Handle(context.Background(), srRecord("sr.comment_added", srTestTeam, srCommentExtra(c.commentType, c.tags))); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(chat.srCalls) != c.wantSends {
				t.Errorf("SR chat sends = %d, want %d", len(chat.srCalls), c.wantSends)
			}
		})
	}
}

// TestDispatcher_Handle_SRCommentAdded_AuthorLookupFailureRetries: a failed
// author classification is the record's error (so it is retried), not a
// silent skip -- the alert is this handler's only reaction.
func TestDispatcher_Handle_SRCommentAdded_AuthorLookupFailureRetries(t *testing.T) {
	chat := &mockGoogleChatSender{}
	d := newSRTestDispatcher(chat, &mockLinkResolver{isCustomerErr: errors.New("entity-service down")})
	err := d.Handle(context.Background(), srRecord("sr.comment_added", srTestTeam, srCommentExtra("comment", `["devops-sm"]`)))
	if err == nil {
		t.Fatal("Handle() error = nil, want the author lookup failure")
	}
	if strings.Contains(err.Error(), "jane@acme.com") {
		t.Errorf("error %q carries the author's email", err)
	}
	if len(chat.srCalls) != 0 {
		t.Errorf("SR chat sends = %d, want 0", len(chat.srCalls))
	}
}

// TestDispatcher_Handle_SR_ChatFailureIsRetried: a failed post is the
// record's error and releases its claim, so the retry posts it -- once.
func TestDispatcher_Handle_SR_ChatFailureIsRetried(t *testing.T) {
	chat := &mockGoogleChatSender{err: errors.New("chat 503")}
	d := newSRTestDispatcher(chat, &mockLinkResolver{})
	record := srRecord("sr.created", srTestTeam, srCreatedExtra)

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("first attempt: Handle() error = nil, want the chat failure")
	}
	if len(d.done) != 0 {
		t.Fatalf("d.done = %v, want the failed post's claim released", d.done)
	}

	chat.err = nil
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("retry: Handle() error = %v", err)
	}
	if got := chat.srStarted.Load(); got != 2 {
		t.Errorf("SR chat attempts = %d, want 2 (one failed, one successful)", got)
	}
	if len(d.done) != 0 {
		t.Errorf("d.done = %v, want empty after the successful retry", d.done)
	}
}

// TestDispatcher_Handle_SR_RedeliveryDuringSendPostsOnce: a second delivery
// of the same record while the first is still posting (a consumer-group
// rebalance, or the DLQ copy) loses the claim and posts nothing, even when it
// is the record's last chance (NoMoreRetries) -- it must not release the
// winner's claim either.
func TestDispatcher_Handle_SR_RedeliveryDuringSendPostsOnce(t *testing.T) {
	for _, typ := range []string{"sr.created", "sr.acknowledged", "sr.comment_added"} {
		t.Run(typ, func(t *testing.T) {
			extra := map[string]string{
				"sr.created":       srCreatedExtra,
				"sr.acknowledged":  srAckExtra,
				"sr.comment_added": srCommentExtra("comment", `["devops-sm"]`),
			}[typ]
			chat := &mockGoogleChatSender{srBlock: make(chan struct{})}
			d := newSRTestDispatcher(chat, &mockLinkResolver{isCustomer: true})
			record := srRecord(typ, srTestTeam, extra)

			winnerDone := make(chan error, 1)
			go func() { winnerDone <- d.Handle(context.Background(), record) }()
			for chat.srStarted.Load() == 0 {
				runtime.Gosched()
			}

			redelivery := record
			redelivery.Topic = "sre-events-dlq"
			redelivery.NoMoreRetries = true
			if err := d.Handle(context.Background(), redelivery); err != nil {
				t.Fatalf("redelivery: Handle() error = %v, want nil", err)
			}
			d.doneMu.Lock()
			held := d.done[recordBaseKey(record)+"/chat"]
			d.doneMu.Unlock()
			if !held {
				t.Fatal("the redelivery released the claim while the first delivery was still posting")
			}

			close(chat.srBlock)
			if err := <-winnerDone; err != nil {
				t.Fatalf("first delivery: Handle() error = %v", err)
			}
			if got := chat.srStarted.Load(); got != 1 {
				t.Errorf("SR chat sends = %d, want exactly 1", got)
			}
		})
	}
}
