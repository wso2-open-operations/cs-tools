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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeDispatcher records what would have gone to GitHub.
type fakeDispatcher struct {
	owner, repo, eventType string
	payload                map[string]any
	calls                  int
	err                    error
}

func (f *fakeDispatcher) Dispatch(_ context.Context, owner, repo, eventType string, p map[string]any) error {
	f.calls++
	f.owner, f.repo, f.eventType, f.payload = owner, repo, eventType, p
	return f.err
}

func obItem(event string, payload map[string]any) repository.OutboundItem {
	return repository.OutboundItem{
		ID: 1, Event: event, WorkItemID: "cr-1",
		Owner: "wso2", Repository: "choreo", IssueNumber: 42, Payload: payload,
	}
}

// THE CONTRACT WITH THE WORKFLOWS. Each mapped repository runs GitHub Actions
// workflows keyed on these exact event types. Renaming one silently stops the
// integration: GitHub accepts the dispatch and nothing listens for it.
func TestOutbound_EventTypesMatchTheWorkflows(t *testing.T) {
	cases := map[string]string{
		outboundCommentAdded: "servicenow-note",
		outboundCaseClosed:   "servicenow-case-update",
		outboundCaseAssigned: "servicenow-case-update",
		outboundCRCreated:    "servicenow-cr-update",
		outboundCRUpdated:    "servicenow-cr-update",
	}
	for event, want := range cases {
		t.Run(event, func(t *testing.T) {
			d := &fakeDispatcher{}
			if err := NewGithubOutboundService(d).Deliver(context.Background(), obItem(event, nil)); err != nil {
				t.Fatalf("Deliver: %v", err)
			}
			if d.eventType != want {
				t.Errorf("event type = %q, want %q", d.eventType, want)
			}
			if d.owner != "wso2" || d.repo != "choreo" {
				t.Errorf("dispatched to %s/%s, want wso2/choreo", d.owner, d.repo)
			}
		})
	}
}

// The payload is built by the trigger in the workflow's shape and must reach
// GitHub unchanged. A field renamed or reformatted here is a field the
// workflow no longer finds.
func TestOutbound_PayloadPassesThroughUntouched(t *testing.T) {
	in := map[string]any{
		"issue_number": 42,
		"note_text":    "[code]<br><b>Raw</b> ServiceNow markup[/code]",
		"note_type":    "COMMENT",
		"case_number":  "CS0433225",
		"case_sys_id":  "abc-123",
		"sn_user":      "nimal@wso2.com",
	}
	d := &fakeDispatcher{}
	if err := NewGithubOutboundService(d).Deliver(context.Background(), obItem(outboundCommentAdded, in)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	for k, want := range in {
		if got := d.payload[k]; got != want {
			t.Errorf("payload[%q] = %v, want %v", k, got, want)
		}
	}
	// Specifically: the markup is NOT converted here. The workflow strips
	// ServiceNow's [code] markers and HTML itself, and doing it here would
	// change what it receives.
	if d.payload["note_text"] != in["note_text"] {
		t.Error("note_text was transformed; the workflow expects it raw")
	}
}

// GitHub rejects a client_payload with more than ten properties. ServiceNow's
// servicenow-cr-update carried nine; an eleventh cost every CR dispatch a 422.
func TestOutbound_PayloadStaysUnderGithubsTenPropertyLimit(t *testing.T) {
	// The widest payload any trigger builds.
	crPayload := map[string]any{
		"github_issue_number": 42, "cr_number": "CHG1", "cr_sys_id": "id",
		"cr_state": "NEW", "assigned_to": "Nimal", "case_sys_id": "cid",
		"planned_start": "t0", "planned_end": "t1", "action": "created",
	}
	d := &fakeDispatcher{}
	if err := NewGithubOutboundService(d).Deliver(context.Background(),
		obItem(outboundCRCreated, crPayload)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if n := len(d.payload); n > 10 {
		t.Fatalf("client_payload has %d properties; GitHub allows 10", n)
	}
}

// The action is decided by the trigger and passed through untouched.
func TestOutbound_ActionIsNotRewritten(t *testing.T) {
	for _, want := range []string{"created", "state_changed", "dates_updated"} {
		d := &fakeDispatcher{}
		item := obItem(outboundCRUpdated, map[string]any{"action": want})
		if err := NewGithubOutboundService(d).Deliver(context.Background(), item); err != nil {
			t.Fatalf("Deliver: %v", err)
		}
		if got := d.payload["action"]; got != want {
			t.Errorf("action = %v, want %v", got, want)
		}
	}
}

// We dispatch and stop. Commenting, labelling and closing the issue are what
// the workflow does; doing them here as well would duplicate all of it.
func TestOutbound_MakesExactlyOneCall(t *testing.T) {
	d := &fakeDispatcher{}
	if err := NewGithubOutboundService(d).Deliver(context.Background(), obItem(outboundCaseClosed, nil)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if d.calls != 1 {
		t.Errorf("made %d GitHub calls, want exactly 1", d.calls)
	}
}

func TestOutbound_UnknownEventIsPermanent(t *testing.T) {
	d := &fakeDispatcher{}
	err := NewGithubOutboundService(d).Deliver(context.Background(), obItem("no_such_event", nil))
	if !OutboundPermanent(err) {
		t.Fatalf("err = %v, want a permanent failure", err)
	}
	if d.calls != 0 {
		t.Error("dispatched an unknown event")
	}
}

func TestOutbound_RowWithoutARepositoryIsPermanent(t *testing.T) {
	d := &fakeDispatcher{}
	item := obItem(outboundCommentAdded, nil)
	item.Owner = ""
	if err := NewGithubOutboundService(d).Deliver(context.Background(), item); !OutboundPermanent(err) {
		t.Fatalf("err = %v, want a permanent failure", err)
	}
}

func TestOutbound_DispatchFailureIsReturned(t *testing.T) {
	d := &fakeDispatcher{err: errors.New("502 bad gateway")}
	err := NewGithubOutboundService(d).Deliver(context.Background(), obItem(outboundCommentAdded, nil))
	if err == nil {
		t.Fatal("want the underlying failure")
	}
	if OutboundPermanent(err) {
		t.Error("a 502 should be retried, not abandoned")
	}
}

func TestOutboundBackoff(t *testing.T) {
	prev := time.Duration(0)
	for attempt := 1; attempt <= 8; attempt++ {
		d := OutboundBackoff(attempt)
		if d < prev {
			t.Fatalf("backoff went backwards at attempt %d: %v after %v", attempt, d, prev)
		}
		if d > outboundMaxBackoff {
			t.Fatalf("attempt %d exceeded the cap: %v", attempt, d)
		}
		prev = d
	}
}
