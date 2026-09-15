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

package flows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// changedEvent builds an entity.changed event for a change request whose state
// moved from -> to, with an optional snapshot.
func changedEvent(t *testing.T, entityType, from, to string, snap map[string]any) Event {
	t.Helper()
	payload := events.EntityChangedPayload{
		EntityType: entityType,
		EntityID:   "cr-1",
		Changes:    map[string]map[string]any{},
		Snapshot:   snap,
	}
	if to != "" || from != "" {
		payload.Changes["state"] = map[string]any{"from": from, "to": to}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return Event{Envelope: events.Envelope{
		Type:     events.TypeEntityChanged,
		EntityID: "cr-1",
		Payload:  raw,
	}}
}

// TestCRApprovalNotice_Match pins the trigger. The ServiceNow condition is
// stateCHANGESTO-4^OR-3^OR5^OR0^OR1 -- CHANGES TO, not "is", which is the part
// a port gets wrong most easily: a change request updated for any other reason
// while already sitting in an approval state must not notify again.
func TestCRApprovalNotice_Match(t *testing.T) {
	f := crApprovalNotice{}

	cases := []struct {
		name string
		evt  Event
		want bool
	}{
		{"assess", changedEvent(t, "change_request", "NEW", "ASSESS", nil), true},
		{"authorize", changedEvent(t, "change_request", "ASSESS", "AUTHORIZE", nil), true},
		{"review", changedEvent(t, "change_request", "IMPLEMENT", "REVIEW", nil), true},
		{"customer approval", changedEvent(t, "change_request", "AUTHORIZE", "CUSTOMER_APPROVAL", nil), true},
		{"customer review", changedEvent(t, "change_request", "REVIEW", "CUSTOMER_REVIEW", nil), true},

		{"a state this flow does not cover", changedEvent(t, "change_request", "REVIEW", "CLOSED", nil), false},
		{"scheduled is not an approval state", changedEvent(t, "change_request", "ASSESS", "SCHEDULED", nil), false},
		{"no state change at all", changedEvent(t, "change_request", "", "", nil), false},
		{"already in the state — not a transition", changedEvent(t, "change_request", "ASSESS", "ASSESS", nil), false},
		{"a different entity", changedEvent(t, "case", "NEW", "ASSESS", nil), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Match(tc.evt); got != tc.want {
				t.Errorf("Match = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCRApprovalNotice_MatchIgnoresOtherEventTypes proves the flow is inert for
// every event that is not entity.changed -- the registry runs every flow's
// Match against every record on the bus.
func TestCRApprovalNotice_MatchIgnoresOtherEventTypes(t *testing.T) {
	evt := changedEvent(t, "change_request", "NEW", "ASSESS", nil)
	for _, ty := range []events.Type{
		events.TypeCaseCreated, events.TypeCommentAdded, events.TypeIncidentCreated,
	} {
		evt.Envelope.Type = ty
		if (crApprovalNotice{}).Match(evt) {
			t.Errorf("matched %s, want no match", ty)
		}
	}
}

// TestCRApprovalNotice_MatchOnMalformedPayload proves a payload this flow
// cannot read is skipped rather than panicking the whole registry -- Match runs
// against every record, including ones written by services this one does not
// control.
func TestCRApprovalNotice_MatchOnMalformedPayload(t *testing.T) {
	evt := Event{Envelope: events.Envelope{
		Type:    events.TypeEntityChanged,
		Payload: json.RawMessage(`{"entityType": 12345}`),
	}}
	if (crApprovalNotice{}).Match(evt) {
		t.Error("expected no match on a payload that fails to decode")
	}
}

// TestCRSubject pins both subject shapes verbatim against the strings read out
// of ServiceNow's sys_element_mapping rows.
func TestCRSubject(t *testing.T) {
	cases := []struct{ number, suffix, team, want string }{
		{
			"CHG0031234", "Request for approval - Customer Review", "",
			"[WSO2 Support] [CR] (CHG0031234) Request for approval - Customer Review",
		},
		{
			"CHG0031234", "Request for Approval - Implementation", "",
			"[WSO2 Support] [CR] (CHG0031234) Request for Approval - Implementation",
		},
		{
			"CHG0031234", "Request for CAB approval - Authorize", "Choreo",
			"[WSO2 Support] [CR][Choreo] (CHG0031234) Request for CAB approval - Authorize",
		},
		{
			"CHG0031234", "Request for approval - Review", "MS",
			"[WSO2 Support] [CR][MS] (CHG0031234) Request for approval - Review",
		},
	}
	for _, tc := range cases {
		if got := crSubject(tc.number, tc.suffix, tc.team); got != tc.want {
			t.Errorf("crSubject(%q, %q, %q)\n got %q\nwant %q", tc.number, tc.suffix, tc.team, got, tc.want)
		}
	}
}

// TestCRTeamFromGitReference pins the If / Else If / Else chain, including its
// fallback: an unrecognised or absent reference is MS by design.
func TestCRTeamFromGitReference(t *testing.T) {
	cases := map[string]string{
		"https://github.com/wso2-enterprise/choreo-apis": "Choreo",
		"CHOREO-1234":                   "Choreo",
		"asgardeo/console":              "Asgardeo",
		"https://github.com/x/Asgardeo": "Asgardeo",
		"something-else":                "MS",
		"":                              "MS",
	}
	for ref, want := range cases {
		if got := crTeamFromGitReference(ref); got != want {
			t.Errorf("crTeamFromGitReference(%q) = %q, want %q", ref, got, want)
		}
	}
}

// TestCRApprovalStatesCoverTheTrigger is the guard that the branch table and
// the ServiceNow trigger condition stay in agreement. The original fires on
// exactly five states; a sixth added here without updating the trigger -- or
// one dropped -- is a silent behaviour change.
func TestCRApprovalStatesCoverTheTrigger(t *testing.T) {
	want := map[string]events.CRApprovalAudience{
		"ASSESS":            events.CRAudienceInternal,
		"AUTHORIZE":         events.CRAudienceInternal,
		"REVIEW":            events.CRAudienceInternal,
		"CUSTOMER_APPROVAL": events.CRAudienceCustomer,
		"CUSTOMER_REVIEW":   events.CRAudienceCustomer,
	}
	if len(crApprovalStates) != len(want) {
		t.Fatalf("branch table has %d states, the trigger covers %d", len(crApprovalStates), len(want))
	}
	for state, audience := range want {
		branch, ok := crApprovalStates[state]
		if !ok {
			t.Errorf("state %s missing from the branch table", state)
			continue
		}
		if branch.audience != audience {
			t.Errorf("state %s: audience %q, want %q", state, branch.audience, audience)
		}
		// Every internal branch names a group; no customer branch does.
		if audience == events.CRAudienceInternal && branch.group == "" {
			t.Errorf("state %s: internal branch with no approval group", state)
		}
		if audience == events.CRAudienceCustomer && branch.group != "" {
			t.Errorf("state %s: customer branch should name no group, got %q", state, branch.group)
		}
		if branch.suffix == "" {
			t.Errorf("state %s: empty subject suffix", state)
		}
	}
}

// TestNormaliseAddresses proves one person in two approval groups is notified
// once -- ServiceNow looped per recipient and could send duplicates.
func TestNormaliseAddresses(t *testing.T) {
	got := normaliseAddresses([]string{"B@wso2.com", "a@wso2.com", "", "  b@wso2.com  ", "a@wso2.com"})
	want := []string{"a@wso2.com", "b@wso2.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestCRApprovalNotice_NotRegistered guards the double-fire rule: registering
// this flow is a paired change with disabling its ServiceNow counterpart in the
// same commit (CLAUDE.md). ServiceNow still owns these notifications, so the
// flow must stay out of All() until that pairing happens -- and until
// csm-notification-service can consume what it publishes.
func TestCRApprovalNotice_NotRegistered(t *testing.T) {
	for _, f := range All() {
		if f.Key() == (crApprovalNotice{}).Key() {
			t.Fatal("cr_approval_notice is registered: registering it requires disabling the " +
				"ServiceNow flow in the same commit, and a csm-notification-service consumer " +
				"for change_request.approval_requested")
		}
	}
}

// TestBuildNoticeAppliesDebugRecipients proves EMAIL_DEBUG_RECIPIENTS replaces
// the real audience rather than adding to it, and — the part that matters — that
// it does NOT turn a would-be-silent event into mail.
func TestBuildNoticeAppliesDebugRecipients(t *testing.T) {
	t.Run("replaces the resolved audience entirely", func(t *testing.T) {
		real := []string{"devops-a@wso2.com", "devops-b@wso2.com"}
		debug := []string{"sasmitha@wso2.com"}

		got := applyDebugRecipients(real, debug)
		if len(got) != 1 || got[0] != "sasmitha@wso2.com" {
			t.Fatalf("got %v, want exactly [sasmitha@wso2.com] — the real audience must be replaced, not appended", got)
		}
	})

	t.Run("an empty override leaves the real audience alone", func(t *testing.T) {
		real := []string{"devops-a@wso2.com"}
		got := applyDebugRecipients(real, nil)
		if len(got) != 1 || got[0] != "devops-a@wso2.com" {
			t.Fatalf("got %v, want the real audience unchanged", got)
		}
	})

	t.Run("no real recipients stays silent even with an override set", func(t *testing.T) {
		got := applyDebugRecipients(nil, []string{"sasmitha@wso2.com"})
		if len(got) != 0 {
			t.Fatalf("got %v, want none — debug mode must not invent a notice nobody would have received", got)
		}
	})
}

// fakeRecipients answers the two audience lookups without a database.
type fakeRecipients struct {
	group    map[string][]string
	contacts map[string][]string
	askedFor string
}

func (f *fakeRecipients) GroupMemberEmails(_ context.Context, team string) ([]string, error) {
	f.askedFor = "group:" + team
	return f.group[team], nil
}

func (f *fakeRecipients) ProjectContactEmails(_ context.Context, projectID string) ([]string, error) {
	f.askedFor = "project:" + projectID
	return f.contacts[projectID], nil
}

// fakeCRs stands in for the database read the notice needs. Its zero value
// answers with an empty record, which is exactly what a deleted change request
// looks like.
type fakeCRs struct {
	details ChangeRequestDetails
	err     error
	askedID string
}

func (f *fakeCRs) ChangeRequestDetails(_ context.Context, id string) (ChangeRequestDetails, error) {
	f.askedID = id
	return f.details, f.err
}

// fakeProducer captures the published bytes instead of writing to a bus.
type fakeProducer struct{ published [][]byte }

func (p *fakeProducer) Publish(_ context.Context, _, value []byte) error {
	p.published = append(p.published, value)
	return nil
}

// runAndDecode runs the flow and returns the single notice it published, or
// nil when it published nothing.
func runAndDecode(t *testing.T, evt Event, deps Deps, prod *fakeProducer) *events.CRApprovalRequestedPayload {
	t.Helper()
	if err := (crApprovalNotice{}).Run(context.Background(), evt, deps); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prod.published) == 0 {
		return nil
	}
	if len(prod.published) != 1 {
		t.Fatalf("published %d events, want exactly 1 — the port collapses ServiceNow's per-recipient loop into one notice", len(prod.published))
	}
	var env events.Envelope
	if err := json.Unmarshal(prod.published[0], &env); err != nil {
		t.Fatalf("published value is not an envelope: %v", err)
	}
	if env.Type != events.TypeCRApprovalRequested {
		t.Fatalf("published type = %q, want %q", env.Type, events.TypeCRApprovalRequested)
	}
	var notice events.CRApprovalRequestedPayload
	if err := json.Unmarshal(env.Payload, &notice); err != nil {
		t.Fatalf("payload is not a CR notice: %v", err)
	}
	return &notice
}

// TestCRApprovalNotice_RunPublishesInternalNotice pins the whole internal
// branch: which audience is asked for, and every field the sending service
// then relies on.
func TestCRApprovalNotice_RunPublishesInternalNotice(t *testing.T) {
	rec := &fakeRecipients{group: map[string][]string{
		"Devops Review": {"B@wso2.com", "a@wso2.com", "a@wso2.com"},
	}}
	crs := &fakeCRs{details: ChangeRequestDetails{
		Number:        "CHG0031234",
		GitReference:  "https://github.com/wso2/choreo-deploy",
		RequesterName: "Sasmitha",
		ProjectID:     "proj-9",
		ProjectName:   "Acme Cloud",
	}}
	prod := &fakeProducer{}
	evt := changedEvent(t, "change_request", "NEW", "REVIEW", nil)

	notice := runAndDecode(t, evt, Deps{Recipients: rec, ChangeRequests: crs, Producer: prod}, prod)
	if notice == nil {
		t.Fatal("published nothing, want an internal approval notice")
	}
	if rec.askedFor != "group:Devops Review" {
		t.Errorf("resolved %q, want the Devops Review group — REVIEW is an internal branch", rec.askedFor)
	}
	if want := "[WSO2 Support] [CR][Choreo] (CHG0031234) Request for approval - Review"; notice.Subject != want {
		t.Errorf("subject = %q, want %q", notice.Subject, want)
	}
	// Lower-cased, de-duplicated, sorted: one person in two groups is told once.
	if got := strings.Join(notice.Recipients, ","); got != "a@wso2.com,b@wso2.com" {
		t.Errorf("recipients = %q, want %q", got, "a@wso2.com,b@wso2.com")
	}
	if notice.Audience != events.CRAudienceInternal {
		t.Errorf("audience = %q, want internal", notice.Audience)
	}
	if notice.ProjectID != "proj-9" {
		t.Errorf("projectId = %q, want proj-9 — the sending service needs it to build a portal link", notice.ProjectID)
	}
}

// TestCRApprovalNotice_RunPublishesCustomerNotice covers the other branch: the
// project's contacts, and a subject with no team in it.
func TestCRApprovalNotice_RunPublishesCustomerNotice(t *testing.T) {
	rec := &fakeRecipients{contacts: map[string][]string{
		"proj-9": {"contact@acme.example"},
	}}
	crs := &fakeCRs{details: ChangeRequestDetails{
		Number:       "CHG0031234",
		GitReference: "https://github.com/wso2/choreo-deploy",
		ProjectID:    "proj-9",
	}}
	prod := &fakeProducer{}
	evt := changedEvent(t, "change_request", "REVIEW", "CUSTOMER_REVIEW", nil)

	notice := runAndDecode(t, evt, Deps{Recipients: rec, ChangeRequests: crs, Producer: prod}, prod)
	if notice == nil {
		t.Fatal("published nothing, want a customer approval notice")
	}
	if rec.askedFor != "project:proj-9" {
		t.Errorf("resolved %q, want the project's contacts", rec.askedFor)
	}
	// No [Choreo] despite the git reference: the customer subflow never
	// carried the team.
	if want := "[WSO2 Support] [CR] (CHG0031234) Request for approval - Customer Review"; notice.Subject != want {
		t.Errorf("subject = %q, want %q", notice.Subject, want)
	}
	if notice.Team != "" || notice.GroupName != "" {
		t.Errorf("team=%q groupName=%q, want both empty on a customer notice", notice.Team, notice.GroupName)
	}
}

// TestCRApprovalNotice_RunSilentWithNoRecipients: an approval group with no
// members publishes nothing at all, rather than a notice addressed to nobody.
func TestCRApprovalNotice_RunSilentWithNoRecipients(t *testing.T) {
	prod := &fakeProducer{}
	evt := changedEvent(t, "change_request", "NEW", "ASSESS", nil)

	notice := runAndDecode(t, evt, Deps{
		Recipients:           &fakeRecipients{},
		ChangeRequests:       &fakeCRs{details: ChangeRequestDetails{Number: "CHG0031234"}},
		Producer:             prod,
		EmailDebugRecipients: []string{"sasmitha@wso2.com"},
	}, prod)
	if notice != nil {
		t.Fatalf("published %+v, want nothing — an empty group must stay silent even with a debug override set", notice)
	}
}

// TestCRApprovalNotice_RunIgnoresTheSnapshot is the regression guard for the
// bug this reader exists to fix.
//
// The outbox trigger writes to_jsonb(NEW) of the table that changed, so the
// snapshot on a change_request row change holds change_request's own columns in
// snake_case — never "number" (that is on work_item), never "projectId", never
// "gitReference". A flow reading those keys gets "" for all of them and
// publishes a notice with an empty subject; the customer branch additionally
// resolves ProjectContactEmails("") and goes silent, so a whole audience is
// never told anything. The snapshot here is deliberately populated with the
// wrong-shaped keys to prove none of them reach the notice.
func TestCRApprovalNotice_RunIgnoresTheSnapshot(t *testing.T) {
	rec := &fakeRecipients{group: map[string][]string{"CAB Approval": {"cab@wso2.com"}}}
	crs := &fakeCRs{details: ChangeRequestDetails{
		Number:       "CHG-FROM-DB",
		GitReference: "https://github.com/wso2/asgardeo-x",
		ProjectID:    "proj-from-db",
	}}
	prod := &fakeProducer{}
	evt := changedEvent(t, "change_request", "ASSESS", "AUTHORIZE", map[string]any{
		"number":       "CHG-FROM-SNAPSHOT",
		"gitReference": "https://github.com/wso2/choreo-x",
		"projectId":    "proj-from-snapshot",
	})

	notice := runAndDecode(t, evt, Deps{Recipients: rec, ChangeRequests: crs, Producer: prod}, prod)
	if notice == nil {
		t.Fatal("published nothing, want an AUTHORIZE notice")
	}
	if crs.askedID != "cr-1" {
		t.Errorf("read change request %q, want the entity the outbox row names", crs.askedID)
	}
	if notice.Number != "CHG-FROM-DB" {
		t.Errorf("number = %q, want the record's — the snapshot cannot carry it", notice.Number)
	}
	if notice.Team != "Asgardeo" {
		t.Errorf("team = %q, want Asgardeo (from the record's git reference)", notice.Team)
	}
	if notice.ProjectID != "proj-from-db" {
		t.Errorf("projectId = %q, want the record's", notice.ProjectID)
	}
}

// TestCRApprovalNotice_RunOnDeletedChangeRequest: the record can vanish between
// the outbox row being written and this running. The reader reports a zero
// value, and the flow must not treat that as a fault to retry forever.
func TestCRApprovalNotice_RunOnDeletedChangeRequest(t *testing.T) {
	prod := &fakeProducer{}
	evt := changedEvent(t, "change_request", "NEW", "CUSTOMER_APPROVAL", nil)

	notice := runAndDecode(t, evt, Deps{
		Recipients:     &fakeRecipients{},
		ChangeRequests: &fakeCRs{},
		Producer:       prod,
	}, prod)
	if notice != nil {
		t.Fatalf("published %+v, want nothing for a change request that no longer exists", notice)
	}
}
