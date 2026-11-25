package slaengine

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeCREHeads is a CREHeadResolver test double.
type fakeCREHeads struct {
	emails []string
	err    error
	calls  int
}

func (f *fakeCREHeads) CREHeadEmails(context.Context) ([]string, error) {
	f.calls++
	return f.emails, f.err
}

// creHeadTestEngine is one 50% response tier due for case-1, with state as
// entity-service's answer and meta as the engine's own Redis copy.
func creHeadTestEngine(t *testing.T, state clockState, meta ClockMeta, heads *fakeCREHeads) (*Engine, *fakeEmail) {
	t.Helper()
	st := newFakeStore()
	email := &fakeEmail{}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.email = email
	if heads != nil {
		e.SetCREHeads(heads)
	}
	past := time.Now().Add(-time.Minute)
	if state.Found {
		state.IsActive, state.Stage, state.StartedOn, state.BusinessElapsedPercent = true, "IN_PROGRESS", &past, 50
	}
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{"case-1|response": state}}
	meta.CaseNumber, meta.StartedAt = "CS0001", past
	st.clocks["case-1|response"] = meta
	st.wake[wakeMember("case-1", "response", 50)] = past
	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	return e, email
}

// TestTick_BreachEmail_NoABT_UnassignedCase_EmailsTheCREHead is the reported
// gap: an unassigned case on an account with no ABT sent its at-risk email
// to nobody. The CRE head now gets the team email, told why.
func TestTick_BreachEmail_NoABT_UnassignedCase_EmailsTheCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{Found: true}, ClockMeta{}, heads)

	if len(email.calls) != 1 {
		t.Fatalf("email calls = %+v, want exactly 1 (the CRE head)", email.calls)
	}
	if got := email.calls[0].to; !slices.Equal(got, []string{"cre-head@example.com"}) {
		t.Errorf("email to = %v, want [cre-head@example.com]", got)
	}
	if !strings.Contains(email.calls[0].htmlBody, "No ABT assigned") {
		t.Error("CRE head email does not say the case has no ABT")
	}
}

// TestTick_BreachEmail_NoABT_AssignedCase_EmailsAssigneeAndCREHead confirms
// the head is added on top of the assignee, not instead of them.
func TestTick_BreachEmail_NoABT_AssignedCase_EmailsAssigneeAndCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{Found: true, AssigneeName: "Jane", AssigneeEmail: "jane@example.com"}, ClockMeta{}, heads)

	if len(email.calls) != 2 {
		t.Fatalf("email calls = %+v, want 2 (assignee + CRE head)", email.calls)
	}
	if got := email.calls[0].to; !slices.Equal(got, []string{"jane@example.com"}) {
		t.Errorf("first email to = %v, want the assignee", got)
	}
	if got := email.calls[1].to; !slices.Equal(got, []string{"cre-head@example.com"}) {
		t.Errorf("second email to = %v, want the CRE head", got)
	}
}

// TestTick_BreachEmail_NoABT_Onboarding_EmailsOnboardingTeamAndCREHead: an
// onboarding-stage project already mailed the onboarding team; the CRE head
// joins that email, since nobody on the CRE side heard otherwise.
func TestTick_BreachEmail_NoABT_Onboarding_EmailsOnboardingTeamAndCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{Found: true, OnboardingTeamEmail: "onboarding@example.com", ProjectOnboardingStatus: "IN_PROGRESS"}, ClockMeta{}, heads)

	if len(email.calls) != 1 {
		t.Fatalf("email calls = %+v, want 1 team email", email.calls)
	}
	if got := email.calls[0].to; !slices.Equal(got, []string{"onboarding@example.com", "cre-head@example.com"}) {
		t.Errorf("team email to = %v, want [onboarding@example.com cre-head@example.com]", got)
	}
}

// TestTick_BreachEmail_ABTWithoutGroupEmail_EmailsTheCREHead: an ABT with no
// group_email reaches no CRE inbox either, so the head stands in; the Team
// row keeps the team's name.
func TestTick_BreachEmail_ABTWithoutGroupEmail_EmailsTheCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{Found: true}, ClockMeta{Team: "Team Nova"}, heads)

	if len(email.calls) != 1 || !slices.Equal(email.calls[0].to, []string{"cre-head@example.com"}) {
		t.Fatalf("email calls = %+v, want 1 to the CRE head", email.calls)
	}
	if body := email.calls[0].htmlBody; !strings.Contains(body, "Team Nova") || strings.Contains(body, "No ABT assigned") {
		t.Error("Team row should name the case's team, not claim it has no ABT")
	}
}

// TestTick_BreachEmail_TeamHasGroupEmail_DoesNotAskForTheCREHead: the normal
// case is unchanged and costs no extra lookup.
func TestTick_BreachEmail_TeamHasGroupEmail_DoesNotAskForTheCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{Found: true, TeamEmail: "team-nova@example.com"}, ClockMeta{Team: "Team Nova"}, heads)

	if heads.calls != 0 {
		t.Errorf("CRE head lookups = %d, want 0", heads.calls)
	}
	if len(email.calls) != 1 || !slices.Equal(email.calls[0].to, []string{"team-nova@example.com"}) {
		t.Errorf("email calls = %+v, want 1 to the team only", email.calls)
	}
}

// TestTick_BreachEmail_ClockStateNotFound_DoesNotEmailTheCREHead: a lookup
// that found nothing (or failed) says nothing about the case's team, so it
// must not be read as "no ABT" -- otherwise an entity-service outage would
// send the head every case's alert.
func TestTick_BreachEmail_ClockStateNotFound_DoesNotEmailTheCREHead(t *testing.T) {
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	_, email := creHeadTestEngine(t, clockState{}, ClockMeta{}, heads)

	if heads.calls != 0 || len(email.calls) != 0 {
		t.Errorf("CRE head lookups = %d, email calls = %+v; want none", heads.calls, email.calls)
	}
}

// TestTick_BreachEmail_CREHeadLookupFails_StillEmailsTheAssignee: a failed
// head lookup costs only the head's copy.
func TestTick_BreachEmail_CREHeadLookupFails_StillEmailsTheAssignee(t *testing.T) {
	heads := &fakeCREHeads{err: errors.New("entity-service down")}
	_, email := creHeadTestEngine(t, clockState{Found: true, AssigneeEmail: "jane@example.com"}, ClockMeta{}, heads)

	if len(email.calls) != 1 || !slices.Equal(email.calls[0].to, []string{"jane@example.com"}) {
		t.Errorf("email calls = %+v, want 1 to the assignee only", email.calls)
	}
}

// A token or transport error's text can carry the upstream response body, so
// the failed lookup logs only the error's type.
func TestTick_BreachEmail_CREHeadLookupFails_LogsTheTypeNotTheText(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	heads := &fakeCREHeads{err: errors.New(`oauth2: cannot fetch token: 400 Response: {"secret-body":1}`)}
	creHeadTestEngine(t, clockState{Found: true}, ClockMeta{}, heads)

	out := logs.String()
	if !strings.Contains(out, "failed to resolve the cre head") || !strings.Contains(out, "errType=*errors.errorString") {
		t.Fatalf("log = %q, want the failure logged with its error type", out)
	}
	if strings.Contains(out, "secret-body") {
		t.Errorf("log = %q, carries the error's text", out)
	}
}

// TestTick_BreachEmail_NoABT_DebugMode_ShowsTheCREHeadFallback: a no-ABT case
// used to be invisible in EMAIL_DEBUG_MODE (empty label, so skipped); it now
// sends the debug copy naming the head it was meant for.
func TestTick_BreachEmail_NoABT_DebugMode_ShowsTheCREHeadFallback(t *testing.T) {
	st := newFakeStore()
	email := &fakeEmail{}
	e := newTestEngine(st, &fakeChat{}, &fakePublisher{})
	e.email = email
	e.emailDebugMode = true
	e.emailDebugRecipients = []string{"debug@example.com"}
	e.SetCREHeads(&fakeCREHeads{emails: []string{"cre-head@example.com"}})
	past := time.Now().Add(-time.Minute)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"case-1|response": {Found: true, IsActive: true, Stage: "IN_PROGRESS", StartedOn: &past, BusinessElapsedPercent: 50},
	}}
	st.clocks["case-1|response"] = ClockMeta{CaseNumber: "CS0001", StartedAt: past}
	st.wake[wakeMember("case-1", "response", 50)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v, want nil", err)
	}
	if len(email.calls) != 1 || !slices.Equal(email.calls[0].to, []string{"debug@example.com"}) {
		t.Fatalf("email calls = %+v, want 1 redirected to the debug recipients", email.calls)
	}
	if !strings.Contains(email.calls[0].htmlBody, "cre-head@example.com") {
		t.Error("debug email does not show the CRE head it was meant for")
	}
}

// TestTick_IncidentBreach_NoSRETeam_DoesNotEmailTheCREHead: an incident's
// team email goes to its SRE team(s), and sendIncidentBreachAlert blanks
// TeamEmail on purpose -- that is not "no ABT", so the CRE head is never
// asked, even when the incident has no SRE team at all.
func TestTick_IncidentBreach_NoSRETeam_DoesNotEmailTheCREHead(t *testing.T) {
	st := newFakeStore()
	chat := &fakeChat{}
	email := &fakeEmail{}
	e := newTestEngine(st, chat, &fakePublisher{})
	e.email = email
	heads := &fakeCREHeads{emails: []string{"cre-head@example.com"}}
	e.SetCREHeads(heads)
	sreTeamWiring(e, chat, nil, []string{"apollo"}, map[string]string{"apollo": "apollo@example.com"})
	past := time.Now().Add(-5 * time.Hour)
	e.entity = &fakeEntityClockStatus{states: map[string]clockState{
		"inc-1|" + ClockResolution: {Found: true, IsActive: true, Stage: "BREACHED", HasBreached: true, StartedOn: &past, AssigneeEmail: "melan@example.com"},
	}}
	st.clocks["inc-1|"+ClockResolution] = ClockMeta{CaseNumber: "INC0091536", CaseType: CaseTypeIncident, Team: "Choreo Runtime Group", Priority: "CRITICAL", StartedAt: past}
	st.wake[wakeMember("inc-1", ClockResolution, 100)] = past

	if err := e.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if heads.calls != 0 {
		t.Errorf("CRE head lookups = %d, want 0 for an incident", heads.calls)
	}
	if want := []string{"melan@example.com"}; !slices.Equal(incidentRecipients(email.calls), want) {
		t.Errorf("email recipients = %v, want the assignee only", incidentRecipients(email.calls))
	}
}
