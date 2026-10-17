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

package paging

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// The Case Paging rung email (cre.email, section 10.0 of the specification).
//
// One email per rung, sent when the rung opens, on the CRE ladder of a
// customer case only. LEVEL_0, the first responders, get no email: they get
// the first call. Who each rung goes to (agreed 2026-10-13):
//
//	L1    To the case's ABT team lead, Cc the case's ABT team list
//	L2    To every ABT team lead, Cc every ABT team list
//	L3    To the CRE head, Cc cre.email.creTeamList
//	L4    To the CS head,  Cc cre.email.csTeamList
//
// At night (21:00-06:00 IST) L1 and L2 ADD the Americas: L1 also To the
// Americas team leads, L2 also To the America Team lead -- the night calls'
// own rungs -- and both also Cc the Americas team list (agreed 2026-10-15).
// By day L1 is the case team's lead and L2 every ABT lead, where the call
// reaches a few. Every rung of a chain shares one subject ("[Case Alerts]
// S0 CS0099001 not acknowledged"), so a case's emails group into one
// conversation; the body's first line says which rung it is ("S0 alert at L1
// for Atlas ABT Team Lead"), and each email lists the rungs already emailed.
//
// WHY ITS OWN SCHEDULE, NOT A NOTIFIER. A notifier delivers one attempt to
// one person, so an L2 email would have gone out once per lead instead of
// once with everyone on it. And a call plan drops anyone without a phone
// number, and holds every call until the ABT leads' numbers are verified; an
// email needs neither, and riding the calls would have lost it in exactly
// those cases. So the emails are planned beside the calls, from the same
// resolved rungs, and keep their own wake entries on the same clock and the
// same stop: an acknowledgement or a closed case drops them with the calls.
//
// The email follows the service's email switches (EMAIL_SENDING_ENABLED,
// EMAIL_DEBUG_MODE), not CALL_SENDING_ENABLED: turning the phones off must not
// also silence the email, and a debug run must not reach real mailboxes.

// PlannedEmail is one rung's email.
type PlannedEmail struct {
	Level Level
	// At is when the rung opens: the first call of the same rung.
	At time.Time
	To []Recipient
	// Cc is the rung's team mailing lists.
	Cc []string `json:",omitempty"`
	// Greeting is who the email addresses: names, or "ABT Team Leads".
	Greeting string
}

// emailSender is notifications.EmailClient's send, for testability.
type emailSender interface {
	SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error
}

// RungEmailSending is the service-wide email switches the rung email obeys.
type RungEmailSending struct {
	// Enabled is EMAIL_SENDING_ENABLED: false logs each email instead.
	Enabled bool
	// DebugMode and DebugRecipients are EMAIL_DEBUG_MODE and
	// EMAIL_DEBUG_RECIPIENTS: every email goes to the debug list instead, with
	// no Cc, and says who it was meant for.
	DebugMode       bool
	DebugRecipients []string
	// SLADurations is the SLA duration policy entity-service serves
	// (GET /sla-duration-policy, severity word -> clock type -> duration),
	// as the SLA engine fetched it at startup; the email's opening line
	// counts down the "response" clock. Nil (the fetch failed, or a test)
	// falls back to defaultResponseSLA.
	SLADurations map[string]map[string]time.Duration
}

// defaultResponseSLA is WSO2's published response SLA per severity -- the
// values entity-service seeds into sla_duration_policy (migration 0192) --
// used only when the policy could not be fetched, so the email still says
// how long is left.
var defaultResponseSLA = map[string]time.Duration{
	"P0": 15 * time.Minute, "P1": time.Hour, "P2": 4 * time.Hour, "P3": 6 * time.Hour, "P4": 24 * time.Hour,
}

// severityWords is the severity as the SLA policy keys it ("CATASTROPHIC").
var severityWords = map[string]string{"P0": "CATASTROPHIC", "P1": "CRITICAL", "P2": "HIGH", "P3": "MEDIUM", "P4": "LOW"}

// responseSLA is the case severity's response SLA: the fetched policy's,
// else the published default.
func (e *Engine) responseSLA(priority string) (time.Duration, bool) {
	p := NormalisePriority(priority)
	if d, ok := e.emailSending.SLADurations[severityWords[p]]["response"]; ok && d > 0 {
		return d, true
	}
	d, ok := defaultResponseSLA[p]
	return d, ok
}

// rungEmailOpening is the email's first sentence: the case is still open, and
// how long is left on its response SLA, counted from when the chain started
// (the case's creation, or its severity change, when entity-service restarts
// the clock too).
func (e *Engine) rungEmailOpening(t Trigger, code string, now time.Time) string {
	open := fmt.Sprintf("Case %s (%s) is still in open state", orNone(t.Number), code)
	sla, ok := e.responseSLA(t.Priority)
	if !ok || t.At.IsZero() {
		return open + "."
	}
	left := t.At.Add(sla).Sub(now)
	if left <= 0 {
		return open + " and the response SLA was breached " + shortDuration(-left) + " ago."
	}
	return open + " and response SLA remaining time is " + shortDuration(left) + "."
}

// WithRungEmail gives the engine its email sender. Without it, cre.email does
// nothing (and main says so at startup).
func (e *Engine) WithRungEmail(sender *notifications.EmailClient, sending RungEmailSending) *Engine {
	// Not assigned when nil, so the interface field stays a true nil.
	if sender != nil {
		e.emails = sender
	}
	e.emailSending = sending
	return e
}

// emailWakeTag marks a wake member as a rung email rather than a call. It sits
// between the ladder key and the index ("<caseId>|email|<n>"), so
// parseWakeMember still splits it, and the ladder key comes back with the tag.
const emailWakeTag = "|email"

func emailWakeMember(incidentID string, index int) string {
	return wakeMember(incidentID+emailWakeTag, index)
}

// emailGiveUpAfter bounds the retries of an email the email service keeps
// failing transiently. Without a bound the ladder could never settle: it runs
// until every call and every email is done.
const emailGiveUpAfter = 15 * time.Minute

// rungEmailsOn reports whether this engine emails the rungs of t.
func (e *Engine) rungEmailsOn(t Trigger) bool {
	return e.cfg.Kind == LadderCRE && e.cfg.Ladder.Email.Enabled && e.emails != nil && t.isCase()
}

// recordingResolver keeps what each rung resolved to while BuildPlan asks, so
// the emails reach the same people as the calls without asking again. Every
// person is kept, including those the call plan then drops for want of a
// number.
type recordingResolver struct {
	Resolver
	got map[Level][]Recipient
}

func (r *recordingResolver) Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	out, err := r.Resolver.Resolve(ctx, level, rc)
	if err == nil {
		r.got[level] = append([]Recipient(nil), out...)
	}
	return out, err
}

// RuleFor passes the wrapped resolver's rule through, so BuildPlan stamps the
// same rule and keeps the same LEVEL_0 as it would without the recording.
func (r *recordingResolver) RuleFor(rc RoutingContext) (Rule, bool) {
	if n, ok := r.Resolver.(ruleNamer); ok {
		return n.RuleFor(rc)
	}
	return Rule{}, false
}

// baseResolver is the resolver under any phone-filling wrapper: the emails
// need people, not numbers.
func baseResolver(r Resolver) Resolver {
	if u, ok := r.(interface{ Unwrap() Resolver }); ok {
		return u.Unwrap()
	}
	return r
}

// emailRungs are the rungs that get an email, in order.
var emailRungs = []Level{Level1, Level2, Level3, Level4}

// rungOpenTimes is when each rung of a freshly built plan opens. Every rung
// the schedule has appears either as calls or as an issue (nobody resolved,
// nobody with a number), both stamped with its opening time, so the emails
// keep the rung's clock even when it places no call.
func rungOpenTimes(plan Plan) map[Level]time.Time {
	opens := map[Level]time.Time{}
	note := func(l Level, at time.Time) {
		if at.IsZero() {
			return
		}
		if cur, ok := opens[l]; !ok || at.Before(cur) {
			opens[l] = at
		}
	}
	for _, c := range plan.Calls {
		note(c.Level, c.At)
	}
	for _, i := range plan.Issues {
		note(i.Level, i.At)
	}
	return opens
}

// planEmails adds the rung emails to a plan just built from rec. Called before
// applySafety, which may hold or trim the calls but not the emails -- except
// safety.maxLevel, which is honoured here: a trial that keeps the ladder away
// from the heads keeps the email away from them too.
func (e *Engine) planEmails(ctx context.Context, plan *Plan, got map[Level][]Recipient) {
	t := plan.Trigger
	opens := rungOpenTimes(*plan)
	maxLevel := Level4
	if m := e.cfg.Ladder.Safety.MaxLevel; m != nil {
		maxLevel = Level(*m)
	}
	night := isNightShift(t.Routing.Shift)
	for _, level := range emailRungs {
		at, ok := opens[level]
		if !ok || level > maxLevel {
			continue
		}
		to, greeting := e.rungEmailTo(ctx, t, level, night, got[level])
		if len(to) == 0 {
			slog.WarnContext(ctx, "escalation: rung email has nobody to go to; skipped",
				"incidentId", t.IncidentID, "level", level.String(), "night", night)
			continue
		}
		if greeting == "" {
			greeting = upperFirst(emailRungAudience(level, night, t.Team))
		}
		plan.Emails = append(plan.Emails, PlannedEmail{
			Level: level, At: at, To: to, Cc: ccFor(to, e.rungEmailLists(t, level, night)), Greeting: greeting,
		})
	}
}

// rungEmailTo is who a rung's email is addressed to, and the greeting.
//
// By day L1 is the case's own ABT team lead and L2 every ABT team lead, where
// the call reaches a few; each falls back to the call's rung when the Team
// Schedule cannot say. At night the call's own rung is added to them -- the
// Americas team leads at L1, the America Team lead at L2 -- never swapped in.
// L3 and L4 are the call's rung, the heads.
func (e *Engine) rungEmailTo(ctx context.Context, t Trigger, level Level, night bool, rung []Recipient) ([]Recipient, string) {
	var day []Recipient
	var dayGreeting string
	switch level {
	case Level1:
		day = e.caseTeamLead(ctx, t)
		dayGreeting = joinNames(day)
	case Level2:
		day = e.abtLeads(ctx, t)
		if len(day) > 0 {
			dayGreeting = "ABT Team Leads"
		}
	default:
		to := uniqueRecipients(rung)
		return to, joinNames(to)
	}
	if !night {
		if len(day) > 0 {
			return day, dayGreeting
		}
		to := uniqueRecipients(rung)
		return to, joinNames(to)
	}
	extra := uniqueRecipients(rung)
	extraGreeting := joinNames(extra)
	if level == Level1 && len(extra) > 1 {
		extraGreeting = "Americas Team Leads"
	}
	return uniqueRecipients(append(append([]Recipient(nil), day...), extra...)), joinNonEmpty(dayGreeting, extraGreeting)
}

// caseTeamLead is the case's own ABT team lead, or nobody when the Team
// Schedule cannot say.
func (e *Engine) caseTeamLead(ctx context.Context, t Trigger) []Recipient {
	tl, ok := baseResolver(e.resolver).(TeamLeadResolver)
	if !ok || strings.TrimSpace(t.Routing.AssignedCRETeam) == "" {
		return nil
	}
	lead, err := tl.TeamLead(ctx, t.Routing.AssignedCRETeam)
	if err != nil {
		slog.WarnContext(ctx, "escalation: could not read the case's team lead for the L1 email; emailing the L1 call's people",
			"incidentId", t.IncidentID, "err", err)
	}
	return uniqueRecipients(lead)
}

// abtLeads is every ABT team lead, or nobody when the Team Schedule cannot say.
func (e *Engine) abtLeads(ctx context.Context, t Trigger) []Recipient {
	lp, ok := baseResolver(e.resolver).(LeadPoolResolver)
	if !ok {
		return nil
	}
	pool, err := lp.LeadPool(ctx)
	if err != nil {
		slog.WarnContext(ctx, "escalation: could not read the ABT team leads for the L2 email; emailing the L2 call's people",
			"incidentId", t.IncidentID, "err", err)
	}
	return uniqueRecipients(pool)
}

// joinNonEmpty is "a and b", or whichever is not empty.
func joinNonEmpty(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " and " + b
}

// rungEmailLists is the mailing lists a rung's email Cc's: the case team's
// at L1, every ABT team's at L2, plus the Americas team's at night; the CRE
// and CS team lists at L3 and L4.
func (e *Engine) rungEmailLists(t Trigger, level Level, night bool) []string {
	m := e.cfg.Ladder.Email
	var out []string
	switch level {
	case Level3:
		return []string{m.CRETeamList}
	case Level4:
		return []string{m.CSTeamList}
	case Level1:
		out = []string{e.teamList(e.sreTeamKey(t.Routing.AssignedCRETeam))}
	default:
		out = e.abtTeamLists()
	}
	if night {
		out = append(out, e.teamList(e.americasKey()))
	}
	return out
}

// americasKey is the Americas team's key; "americas" when the file names none.
func (e *Engine) americasKey() string {
	if k := teamKeyFor(e.cfg.Ladder.Teams.Americas); k != "" {
		return k
	}
	return "americas"
}

// teamList is one team's mailing list, or "".
func (e *Engine) teamList(key string) string {
	for k, addr := range e.cfg.Ladder.Email.TeamLists {
		if key != "" && teamKeyFor(k) == key {
			return strings.TrimSpace(addr)
		}
	}
	return ""
}

// abtTeamLists is every ABT team's mailing list -- every list but the
// Americas team's, and only teams.abts when it is written down. Sorted, so
// the Cc is the same on every send.
func (e *Engine) abtTeamLists() []string {
	abts := e.cfg.Ladder.Teams.ABTs
	americas := e.americasKey()
	var out []string
	for k, addr := range e.cfg.Ladder.Email.TeamLists {
		key := teamKeyFor(k)
		if key == americas || (len(abts) > 0 && !contains(abts, key)) {
			continue
		}
		if a := strings.TrimSpace(addr); a != "" {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

func isNightShift(s Shift) bool { return s == ShiftUSA || s == ShiftUSAWeekend }

// ccFor is the lists, less blanks, repeats and anyone already in To.
func ccFor(to []Recipient, lists []string) []string {
	seen := map[string]bool{}
	for _, r := range to {
		seen[normaliseEmail(r.Email)] = true
	}
	var cc []string
	for _, addr := range lists {
		k := normaliseEmail(addr)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		cc = append(cc, strings.TrimSpace(addr))
	}
	return cc
}

// uniqueRecipients drops people with no address and repeats, keeping order.
func uniqueRecipients(rs []Recipient) []Recipient {
	seen := map[string]bool{}
	out := make([]Recipient, 0, len(rs))
	for _, r := range rs {
		k := normaliseEmail(r.Email)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

// joinNames is "A", "A and B", "A, B and C". A person with no name is left
// out, so a greeting never shows an address.
func joinNames(rs []Recipient) string {
	var names []string
	for _, r := range rs {
		if n := strings.TrimSpace(r.Name); n != "" {
			names = append(names, n)
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// emailRungAudience names who a rung's email is for, as the subject and the
// "Already notified" and "Next escalation" rows say it: "Atlas ABT Team
// Lead", "all ABT Team Leads", "CRE Head", "CS Head"; at night L1 and L2 add
// " and Americas Team Leads" and " and America Team Lead". team is the case's
// team, as it reads.
func emailRungAudience(l Level, night bool, team string) string {
	var a string
	switch l {
	case Level1:
		a = "ABT Team Lead"
		if tm := strings.TrimSpace(team); tm != "" {
			a = tm + " " + a
		}
		if night {
			a += " and Americas Team Leads"
		}
	case Level2:
		a = "all ABT Team Leads"
		if night {
			a += " and America Team Lead"
		}
	case Level3:
		a = "CRE Head"
	case Level4:
		a = "CS Head"
	default:
		a = l.String()
	}
	return a
}

// upperFirst capitalises the first letter, for a name that starts a row.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// severityNames name each priority the way a case's severity reads.
var severityNames = map[string]string{"P0": "Catastrophic", "P1": "Critical", "P2": "High", "P3": "Medium", "P4": "Low"}

// emailSeverity is ("S1", "S1 - Critical") for a case severity, however it
// is spelled; an unknown one comes back as given.
func emailSeverity(priority string) (code, label string) {
	p := NormalisePriority(priority)
	name, ok := severityNames[p]
	if !ok {
		v := strings.TrimSpace(priority)
		return v, v
	}
	code = "S" + strings.TrimPrefix(p, "P")
	return code, code + " – " + name
}

// processDueEmail sends one due rung email. The same order as a call: claim,
// send, record, then drop the wake entry, so a crash after sending sends again
// rather than never.
func (e *Engine) processDueEmail(ctx context.Context, incidentID string, index int, member string) error {
	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for %s: %w", incidentID, err)
	}
	if !found || index >= len(st.Plan.Emails) || st.Cancelled != nil || st.emailDone(index) {
		return e.store.RemoveWakes(ctx, member)
	}
	claim := fmt.Sprintf("%s@%d", member, st.Plan.Trigger.At.UnixNano())
	claimed, err := e.store.ClaimCall(ctx, claim, callClaimTTL)
	if err != nil {
		return fmt.Errorf("escalation: claim due email for %s: %w", incidentID, err)
	}
	if !claimed {
		return nil
	}

	email := st.Plan.Emails[index]
	if err := e.sendRungEmail(ctx, st, index); err != nil {
		giveUp := isPermanent(err) || e.now().After(email.At.Add(emailGiveUpAfter))
		if !giveUp {
			if rerr := e.store.ReleaseCall(ctx, claim); rerr != nil {
				slog.WarnContext(ctx, "escalation: could not release a failed email's claim; it retries when the claim expires",
					"incidentId", incidentID, "err", rerr)
			}
			return fmt.Errorf("escalation: send %s email for %s: %w", email.Level, incidentID, err)
		}
		slog.ErrorContext(ctx, "escalation: rung email could not be sent; not retrying",
			"incidentId", incidentID, "level", email.Level.String(), "err", err)
	}

	// Re-read before writing, as processDue does: an acknowledgement may have
	// landed while the email was in flight.
	fresh, stillRunning, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: reload ladder for %s: %w", incidentID, err)
	}
	if !stillRunning {
		return e.store.RemoveWakes(ctx, member)
	}
	if !sameChain(fresh.Plan.Trigger, st.Plan.Trigger) {
		// A severity change replaced the chain while this email was in
		// flight. The new chain's email at this index has the same wake
		// member and is its own, still to be sent: touch neither.
		return nil
	}
	if fresh.Cancelled != nil || index >= len(fresh.Plan.Emails) {
		return e.store.RemoveWakes(ctx, member)
	}
	fresh.markEmailDone(index)
	if err := e.store.Save(ctx, incidentID, fresh); err != nil {
		return fmt.Errorf("escalation: record sent email for %s: %w", incidentID, err)
	}
	if err := e.store.RemoveWakes(ctx, member); err != nil {
		return fmt.Errorf("escalation: clear sent email for %s: %w", incidentID, err)
	}
	if fresh.AllSettled() {
		// The email was the last thing left; end the ladder as processDue
		// would after its last call.
		e.writeNote(ctx, fresh.Plan, fresh.Placed, fresh.Failed, nil, "")
		e.postUnanswered(ctx, fresh)
		slog.WarnContext(ctx, "escalation: ladder exhausted without acknowledgement",
			"incidentId", incidentID, "priority", fresh.Plan.Trigger.Priority,
			"rule", fresh.Plan.Trigger.Routing.Rule(), "reachedLevel", fresh.ReachedLevel(),
			"placedCalls", fresh.PlacedCount())
		return e.store.Delete(ctx, incidentID)
	}
	return nil
}

// markEmailDone records that rung email i was sent or given up on.
func (s *LadderState) markEmailDone(i int) {
	if len(s.EmailSent) < len(s.Plan.Emails) {
		grown := make([]bool, len(s.Plan.Emails))
		copy(grown, s.EmailSent)
		s.EmailSent = grown
	}
	s.EmailSent[i] = true
}

// sendRungEmail renders and sends rung email i of a running ladder.
func (e *Engine) sendRungEmail(ctx context.Context, st LadderState, i int) error {
	t := st.Plan.Trigger
	email := st.Plan.Emails[i]
	code, severity := emailSeverity(t.Priority)

	opening := e.rungEmailOpening(t, code, e.now())
	night := isNightShift(t.Routing.Shift)
	var notified []string
	for j := 0; j < i; j++ {
		if !st.emailDone(j) {
			continue
		}
		prev := st.Plan.Emails[j]
		audience := emailRungAudience(prev.Level, night, t.Team)
		line := upperFirst(audience)
		// Names only where they are one or two people; every ABT lead by name
		// would be seven, and the label says who they are.
		if prev.Level == Level3 || prev.Level == Level4 || (prev.Level == Level1 && !night) {
			if names := joinNames(prev.To); names != "" {
				line = names + " (" + audience + ")"
			}
		}
		notified = append(notified, line+", "+prev.At.In(IST).Format("15:04 IST"))
	}
	next := ""
	if i+1 < len(st.Plan.Emails) {
		n := st.Plan.Emails[i+1]
		next = upperFirst(emailRungAudience(n.Level, night, t.Team)) + " in " + shortDuration(n.At.Sub(email.At))
	}
	waiting := elapsedSince(t.At, e.now())
	if waiting == "" {
		waiting = "under a minute"
	}

	to := make([]string, 0, len(email.To))
	for _, r := range email.To {
		to = append(to, r.Email)
	}
	cc := email.Cc
	intendedFor := ""
	if e.emailSending.DebugMode {
		intendedFor = strings.Join(append(append([]string(nil), to...), cc...), ", ")
		to, cc = e.emailSending.DebugRecipients, nil
		if len(to) == 0 {
			slog.WarnContext(ctx, "escalation: EMAIL_DEBUG_MODE is on with no EMAIL_DEBUG_RECIPIENTS; rung email not sent",
				"incidentId", t.IncidentID, "level", email.Level.String())
			return nil
		}
	}

	body := notifications.RenderCasePagingEmail(notifications.CasePagingEmailData{
		Greeting:        email.Greeting,
		RungLine:        notifications.CasePagingRungLine(code, int(email.Level), emailRungAudience(email.Level, night, t.Team)),
		SeverityCode:    code,
		Rung:            int(email.Level),
		EscalationLevel: fmt.Sprintf("L%d · %s", int(email.Level), upperFirst(emailRungAudience(email.Level, night, t.Team))),
		Opening:         opening,
		CaseNumber:      t.Number,
		WSO2CaseID:      t.WSO2CaseID,
		Title:           t.Title,
		Severity:        severity,
		Account:         t.Account,
		Team:            t.Team,
		CreatedAt:       istStamp(t.At),
		Waiting:         waiting,
		Notified:        notified,
		NextEscalation:  next,
		CaseURL:         e.links.CaseLink(t.IncidentID),
		IntendedFor:     intendedFor,
	})
	subject := notifications.CasePagingSubject(code, t.Number)

	if !e.emailSending.Enabled {
		slog.InfoContext(ctx, "escalation: email sending disabled (EMAIL_SENDING_ENABLED=false); rung email not sent",
			"incidentId", t.IncidentID, "level", email.Level.String(), "to", len(to), "cc", len(cc))
		return nil
	}
	// Counts only: this service never logs an address.
	slog.InfoContext(ctx, "escalation: sending rung email",
		"incidentId", t.IncidentID, "level", email.Level.String(), "to", len(to), "cc", len(cc),
		"debugRedirect", e.emailSending.DebugMode)
	if err := e.emails.SendEmail(ctx, to, cc, nil, nil, subject, body, nil); err != nil {
		return err
	}
	slog.InfoContext(ctx, "escalation: rung email sent",
		"incidentId", t.IncidentID, "level", email.Level.String(), "to", len(to), "cc", len(cc))
	return nil
}
