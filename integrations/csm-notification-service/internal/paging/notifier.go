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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// How a rung reaches a person.
//
// The ladder's timing, routing and cancellation are all channel-agnostic: what
// changes between a phone call and a chat card is only the last hop. Splitting
// that hop out means a deployment can page by voice, post to a space, or do
// both, without any of the rules above it knowing which.
//
// A WORD ON WHICH TO USE. The specification's ladder is a pager, and a chat
// card does not wake anyone. The initial chat alert to the Incident Monitor
// space already exists (see dispatch.handleIncidentCreated), and the ladder
// exists because that was not enough to get an unattended incident attended
// to overnight. Chat as the only channel is therefore a real reduction in what
// the feature does, whatever its other merits. Where it earns its place is
// alongside the calls, giving the room sight of an escalation climbing, and as
// the only channel that can be exercised end to end without a telephony
// account.

// Channel selects which notifiers a ladder uses.
type Channel string

const (
	// ChannelCall is the specification's own: a voice call per attempt.
	ChannelCall Channel = "call"
	// ChannelChat posts to the incident's Google Chat space instead.
	ChannelChat Channel = "chat"
	// ChannelBoth calls and posts. The call decides the attempt: a call that
	// fails is retried and the card may be posted twice — acceptable, since a
	// duplicate card costs a glance and a missed page costs an incident. A
	// card that fails once the call has gone out is retried on its own, never
	// with the call.
	ChannelBoth Channel = "both"
	// ChannelLog writes what it would have done and does nothing else.
	//
	// It exists because the two channels that reach people both need an
	// account: calls need Twilio and cost money per rung, and chat needs a
	// webhook for a real room that real colleagues are sitting in. Neither is
	// something to point at a test. Without a third option the only way to
	// exercise the whole service — the consumer, the resolver, the Redis
	// state, the wake loop — was to either spend money or interrupt people.
	//
	// It is a channel rather than a global dry-run flag so that it appears in
	// the same place every other delivery decision does, and so a deployment
	// cannot end up half-live: a ladder on this channel reaches nobody, and
	// says so in a line per rung.
	ChannelLog Channel = "log"
)

// ParseChannel resolves the configured channel, defaulting to calls.
//
// Defaulting to calls rather than to whatever is configured is deliberate:
// silently downgrading a pager to a chat message because a webhook happened to
// be set would be the kind of change nobody notices until an incident is
// missed.
func ParseChannel(raw string) (Channel, error) {
	switch Channel(strings.ToLower(strings.TrimSpace(raw))) {
	case "", ChannelCall:
		return ChannelCall, nil
	case ChannelChat:
		return ChannelChat, nil
	case ChannelBoth:
		return ChannelBoth, nil
	case ChannelLog:
		return ChannelLog, nil
	}
	return "", fmt.Errorf("escalation: unknown channel %q; use call, chat, both or log", raw)
}

// Uses reports whether this channel includes the given one.
//
// ChannelBoth means both of the channels that reach somebody — call and chat —
// and deliberately not the log channel. A deployment wanting a written record
// alongside real calls gets it from the execution summary, not from here, and
// a live ladder that also emitted "would notify" lines would read like a dry
// run in the middle of a real page.
func (c Channel) Uses(other Channel) bool {
	if other == ChannelLog {
		return c == ChannelLog
	}
	return c == other || c == ChannelBoth
}

// Reaches reports whether this channel can actually reach a person. It is what
// separates a ladder that pages from one that only writes down what it would
// have done, and it is why BuildPlan does not demand a phone number under
// ChannelLog any more than it does under ChannelChat.
func (c Channel) Reaches() bool { return c != ChannelLog }

// logNotifier is ChannelLog's delivery: one line per call, naming the rung and
// who would have been reached.
//
// It logs the recipient's LogID -- never their name, address or number. The
// name is on the chat card a room can see, but the log ships somewhere else,
// and a test channel is not a reason to start writing people into it.
type logNotifier struct{}

func (logNotifier) Channel() Channel { return ChannelLog }

func (logNotifier) Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error) {
	slog.InfoContext(ctx, "escalation: would notify (channel=log; nobody is being contacted)",
		"incidentId", plan.Trigger.IncidentID,
		"incident", plan.Trigger.Number,
		"priority", plan.Trigger.Priority,
		"rung", call.Level.String(),
		"role", rungRole(call.Level, plan.Trigger.Routing),
		"attempt", call.Ordinal,
		"recipientId", call.Recipient.LogID(),
		"shift", call.Recipient.ShiftCode,
		"hasNumber", call.Recipient.Phone != "",
	)
	return Delivery{Channel: ChannelLog, Status: "logged, not sent"}, nil
}

// Delivery is what a notifier did, for the log line that follows it.
type Delivery struct {
	// Channel is which notifier produced it.
	Channel Channel
	// Ref is the provider's own handle where there is one — a Twilio call
	// sid, which is how an operator finds that call in the console. Chat
	// webhooks return no durable id, so it is empty there.
	Ref string
	// Status is the provider's word for what happened ("queued" for a call),
	// or this package's own where the provider gives none.
	Status string
}

// notifier delivers one attempt of one rung.
type notifier interface {
	// Deliver notifies one recipient. plan is the whole ladder, not just this
	// call, because a channel may need to say where the escalation goes next:
	// a phone call is a moment and needs no such context, while a card sits
	// in a room where the reader cannot see the clock running.
	Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error)
	Channel() Channel
}

// voiceNotifier places the specification's phone call.
type voiceNotifier struct {
	calls   callPlacer
	useSSML bool
}

func (v voiceNotifier) Channel() Channel { return ChannelCall }

func (v voiceNotifier) Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error) {
	t := plan.Trigger
	if call.Recipient.Phone == "" {
		// Only reachable on ChannelBoth: a call-only plan drops these before
		// scheduling, and a chat-only plan never builds this notifier. Here
		// the chat half of the rung still has somewhere to go, so record the
		// miss and let it. Dialling "" would be a provider error about a
		// malformed number, which says nothing true about what happened.
		return Delivery{Channel: ChannelCall, Status: "no number for this recipient"}, nil
	}
	var placed notifications.Call
	var err error
	if v.useSSML {
		placed, err = v.calls.MakeSSMLCall(ctx, call.Recipient.Phone, t.VoiceSpeech())
	} else {
		placed, err = v.calls.MakeCall(ctx, call.Recipient.Phone, t.VoiceMessagePlain())
	}
	if err != nil {
		return Delivery{Channel: ChannelCall}, err
	}
	return Delivery{Channel: ChannelCall, Ref: placed.SID, Status: placed.Status}, nil
}

// chatSender abstracts the Google Chat client for testability.
type chatSender interface {
	SendEscalationAlert(ctx context.Context, a notifications.EscalationAlert) error
	// SendEscalationUnanswered posts the closing message of a ladder nobody
	// acknowledged (sre.unansweredChat).
	SendEscalationUnanswered(ctx context.Context, u notifications.EscalationUnanswered) error
	// SendEscalationHandoff tells an SRE space its incident went to the SMEs
	// (sre.smeHandoffChat).
	SendEscalationHandoff(ctx context.Context, h notifications.EscalationHandoff) error
	// SendEscalationStopped replies in a ladder's thread that it stopped
	// (ackChat).
	SendEscalationStopped(ctx context.Context, s notifications.EscalationStopped) error
	// SendIncidentCreatedAlert posts the "Incident Reported" card an SRE incident's rung replies sit under.
	SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error
	// SendSMEIncidentEscalatedAlert posts the "Incident Escalated" card an SME incident's rung replies sit under.
	SendSMEIncidentEscalatedAlert(ctx context.Context, audience string, a notifications.SMEIncidentEscalatedAlert) error
	// SendIncidentAssignedAlert posts the "Incident Acknowledged." reply that ends an assigned SRE incident's thread.
	SendIncidentAssignedAlert(ctx context.Context, audience string, a notifications.IncidentAssignedAlert) error
	// HasAudienceSpace reports whether GOOGLE_CHAT_SPACES has a room for the
	// audience. Asked before every rung, because the client itself treats an
	// unknown audience as a quiet success.
	HasAudienceSpace(audience string) bool
}

// incidentLinker builds the portal link a card points at.
type incidentLinker interface {
	IncidentLink(incidentID string) string
	CaseLink(caseID string) string
}

// chatNotifier posts a rung to the incident's space.
//
// ONE CARD PER RUNG, not one per attempt. A rung's repeat attempts exist
// because the previous call was not answered — a question a phone can ask and
// a posted message cannot, since the card is still sitting there. A P1 ladder
// is fourteen attempts; fourteen near-identical cards would bury the room in
// the thing meant to alert it, and the escalation a reader needs to see is the
// rung changing, not the reminder repeating.
//
// Later attempts of a rung are reported as delivered, because the rung was
// genuinely notified and the card is still in the room. Nothing is suppressed
// silently: the status says which it was.
type chatNotifier struct {
	chat  chatSender
	links incidentLinker
	// audience is the room every rung card goes to. Empty -- a notifier built
	// as a struct literal, as tests do -- means "Incident Monitor", so the zero
	// value routes the same as an unconfigured deployment.
	audience string
	// reply posts the rung as a short reply under the incident's card, which only its INCIDENT_CHAT_SPACES space has.
	reply bool
}

// roomFor is the audience a card goes to.
func (n chatNotifier) roomFor() string {
	if a := strings.TrimSpace(n.audience); a != "" {
		return a
	}
	return chataudience.IncidentMonitor
}

func (chatNotifier) Channel() Channel { return ChannelChat }

func (n chatNotifier) Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error) {
	// Before anything else, and on every attempt. With no room for the
	// audience the client would report success and post nothing, so the rung
	// was logged "posted" -- and a rung's later attempts said "already posted
	// for this rung" about a card that never existed. Permanent: the
	// configuration will not grow a room on the next tick.
	room := n.roomFor()
	if !n.chat.HasAudienceSpace(room) {
		return Delivery{Channel: ChannelChat}, &undeliverable{
			reason: "NO_CHAT_SPACE",
			detail: fmt.Sprintf("no Google Chat space for audience %q in GOOGLE_CHAT_SPACES", room),
		}
	}
	if call.Ordinal > 1 {
		return Delivery{Channel: ChannelChat, Status: "already posted for this rung"}, nil
	}

	t := plan.Trigger
	// The SRE and SME ladders' own spaces get a parent incident card; an incident space (n.reply) already has its card.
	threaded := !n.reply && t.threadedIncident()
	nextRung, nextIn := plan.NextRungAfter(call)
	portal, portalLabel := "", "View incident"
	if t.isCase() {
		portalLabel = "View case"
	}
	if n.links != nil {
		if t.isCase() {
			portal = n.links.CaseLink(t.IncidentID)
		} else {
			portal = n.links.IncidentLink(t.IncidentID)
		}
	}

	alert := notifications.EscalationAlert{
		Audience:      room,
		Rung:          call.Level.LabelIn(t.Routing.Ladder),
		FirstRung:     call.Level == Level0,
		RungRole:      rungRole(call.Level, t.Routing),
		Attempt:       call.Ordinal,
		Priority:      t.Priority,
		IncidentRef:   t.caseRef(),
		Title:         t.Title,
		RecipientName: call.Recipient.Name,
		Instruction:   t.instruction(false),
		// What the recipient would hear, on the card, because on a chat-only
		// deployment nobody ever hears it.
		VoiceScript: t.VoiceMessagePlain(),
		Rule:        t.Routing.Rule(),
		PortalURL:   portal,
		PortalLabel: portalLabel,
		Elapsed:     elapsedSince(t.At, call.At),
		NextRung:    nextRung,
		NextIn:      nextIn,
		// An incident's rungs reply under its "Incident Reported" card, so the space shows the escalation in that one thread.
		ThreadKey: t.chatThreadKey(),
		Reply:     n.reply || threaded,
		Team:      t.Team,
		// Both stamps in IST, because every shift boundary this ladder routes
		// by is expressed in it. Rendering either in the reader's own zone
		// would put a time outside those hours beside a card naming the
		// window it falls in.
		ReportedAt: istStamp(t.At),
		Firing:     istStamp(call.At),
		Rotation:   rotationOf(plan),
		OnCall:     onCallName(plan),
	}
	parent := threaded && isFirstCall(plan, call) && !slices.Contains(plan.IncidentCards, room)
	if parent {
		var err error
		if t.smeIncident() {
			err = n.chat.SendSMEIncidentEscalatedAlert(ctx, room, smeIncidentCard(plan, portal))
		} else {
			err = n.chat.SendIncidentCreatedAlert(ctx, room, incidentCard(t, portal))
		}
		if err != nil {
			return Delivery{Channel: ChannelChat}, err
		}
	}
	if err := n.chat.SendEscalationAlert(ctx, alert); err != nil {
		if parent {
			// The card is in the room: say so, so the retry posts only the reply.
			return Delivery{Channel: ChannelChat}, &incidentCardPosted{room: room, err: err}
		}
		return Delivery{Channel: ChannelChat}, err
	}
	return Delivery{Channel: ChannelChat, Status: "posted to " + room}, nil
}

// incidentCardPosted is a first rung whose "Incident Reported" card reached room but whose reply did not.
type incidentCardPosted struct {
	room string
	err  error
}

func (e *incidentCardPosted) Error() string { return e.err.Error() }
func (e *incidentCardPosted) Unwrap() error { return e.err }

// postedIncidentCards is every room a failed delivery already posted the incident card to.
func postedIncidentCards(err error) []string {
	switch e := err.(type) {
	case nil:
		return nil
	case *incidentCardPosted:
		return []string{e.room}
	case interface{ Unwrap() []error }:
		var rooms []string
		for _, inner := range e.Unwrap() {
			rooms = append(rooms, postedIncidentCards(inner)...)
		}
		return rooms
	case interface{ Unwrap() error }:
		return postedIncidentCards(e.Unwrap())
	}
	return nil
}

// isFirstCall reports whether call is the plan's first, the one that opens the SRE space's incident thread.
func isFirstCall(plan Plan, call PlannedCall) bool {
	if len(plan.Calls) == 0 {
		return false
	}
	first := plan.Calls[0]
	return first.Level == call.Level && first.Ordinal == call.Ordinal && first.At.Equal(call.At) &&
		first.Recipient.Email == call.Recipient.Email && first.Recipient.Name == call.Recipient.Name
}

// incidentCard is the "Incident Reported" card the SRE space's rung replies sit under, the same card INCIDENT_CHAT_SPACES gets.
func incidentCard(t Trigger, link string) notifications.IncidentCreatedAlert {
	return notifications.IncidentCreatedAlert{
		IncidentID:       t.IncidentID,
		Number:           t.Number,
		Team:             t.Team,
		Priority:         t.Priority,
		ShortDescription: t.Title,
		Service:          t.Service,
		Environment:      t.Environment,
		Category:         t.Category,
		State:            t.State,
		IncidentLink:     link,
	}
}

// smeIncidentCard is the "Incident Escalated" card the SME space's rung replies sit under.
func smeIncidentCard(plan Plan, link string) notifications.SMEIncidentEscalatedAlert {
	t := plan.Trigger
	onCall, onCallRung, onCallAt := "", "", ""
	if len(plan.Calls) > 0 {
		first := plan.Calls[0]
		if onCall = first.Recipient.Name; onCall == "" {
			onCall = first.Level.RoleIn(LadderSME)
		}
		if first.Level != Level0 {
			// L1 is empty: the card names the first tier that is rostered and when it is called.
			onCallRung, onCallAt = first.Level.LabelIn(LadderSME), istStamp(first.At)
		}
	}
	return notifications.SMEIncidentEscalatedAlert{
		IncidentID:       t.IncidentID,
		Number:           t.Number,
		SMETeam:          smeTeamLabel(t.Routing.SMETeam),
		Priority:         t.Priority,
		ShortDescription: t.Title,
		Service:          t.Service,
		State:            t.State,
		EscalatedBy:      t.EscalatedBy,
		EscalatedFrom:    t.EscalatedFrom,
		OnCall:           onCall,
		OnCallRung:       onCallRung,
		OnCallAt:         onCallAt,
		EscalatedAt:      istStamp(t.At),
		IncidentLink:     link,
	}
}

// fanoutChat posts one rung's card to several Chat spaces: an incident with
// no SRE team of its own goes to every space sre.teamlessChats names. It is
// one notifier on purpose. A failed card is retried to every space at once,
// reposting it wherever it landed, so a space that fails while another
// succeeds counts as delivered (the failure is logged), and only a rung no
// space took is retried, transient failures first.
type fanoutChat struct {
	chats []chatNotifier
}

func (fanoutChat) Channel() Channel { return ChannelChat }

func (f fanoutChat) Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error) {
	var posted []string
	var permanent, transient error
	for _, c := range f.chats {
		d, err := c.Deliver(ctx, plan, call)
		if err != nil {
			slog.WarnContext(ctx, "escalation: a Chat space did not take the rung card",
				"incidentId", plan.Trigger.IncidentID, "level", call.Level.String(), "audience", c.roomFor(), "err", err)
			if isPermanent(err) {
				permanent = errors.Join(permanent, err)
			} else {
				transient = errors.Join(transient, err)
			}
			continue
		}
		posted = append(posted, d.Status)
	}
	switch {
	case len(posted) > 0:
		return Delivery{Channel: ChannelChat, Status: strings.Join(posted, "; ")}, nil
	case transient != nil:
		return Delivery{Channel: ChannelChat}, transient
	default:
		return Delivery{Channel: ChannelChat}, permanent
	}
}

// elapsedSince renders how long an incident has gone unattended, which is the
// number that makes a rung feel urgent rather than routine.
func elapsedSince(trigger, at time.Time) string {
	d := at.Sub(trigger).Round(time.Minute)
	if d <= 0 {
		return ""
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// istStamp renders an instant in the zone the shift boundaries are written in.
func istStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(IST).Format("2006-01-02 15:04 IST")
}

// rotationOf is the rota window the first rung was resolved from.
//
// It comes off a recipient rather than off the trigger because the trigger
// only knows the shift the boundaries put it in - LK_EVENING - and not which
// scheduled window that was. Only the resolver that read the rota knows that,
// so this is empty when the rungs came from configuration instead.
func rotationOf(plan Plan) string {
	for _, c := range plan.Calls {
		if c.Level == Level0 && c.Recipient.ShiftCode != "" {
			return c.Recipient.ShiftCode
		}
	}
	return ""
}

// onCallName is who the first rung reached, carried on every later card: by
// LEVEL_3 the question is not only who is being called now, but who was
// supposed to have this an hour ago.
func onCallName(plan Plan) string {
	for _, c := range plan.Calls {
		if c.Level == Level0 && c.Recipient.Name != "" {
			return c.Recipient.Name
		}
	}
	return ""
}
