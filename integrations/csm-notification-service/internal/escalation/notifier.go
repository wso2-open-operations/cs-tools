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

package escalation

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// ChannelBoth calls and posts. A failure of either is a failure of the
	// attempt, so the call is retried and the card may be posted twice —
	// acceptable, since a duplicate card costs a glance and a missed page
	// costs an incident.
	ChannelBoth Channel = "both"
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
	}
	return "", fmt.Errorf("escalation: unknown channel %q; use call, chat or both", raw)
}

// Uses reports whether this channel includes the given one.
func (c Channel) Uses(other Channel) bool { return c == other || c == ChannelBoth }

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
}

// incidentLinker builds the portal link a card points at.
type incidentLinker interface {
	IncidentLink(incidentID string) string
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
	chat           chatSender
	links          incidentLinker
	defaultProduct string
}

func (chatNotifier) Channel() Channel { return ChannelChat }

func (n chatNotifier) Deliver(ctx context.Context, plan Plan, call PlannedCall) (Delivery, error) {
	if call.Ordinal > 1 {
		return Delivery{Channel: ChannelChat, Status: "already posted for this rung"}, nil
	}

	t := plan.Trigger
	nextRung, nextIn := plan.NextRungAfter(call)
	product := t.Routing.Product
	if product == "" {
		product = n.defaultProduct
	}
	portal := ""
	if n.links != nil {
		portal = n.links.IncidentLink(t.IncidentID)
	}

	alert := notifications.EscalationAlert{
		Product:       product,
		Rung:          call.Level.String(),
		RungRole:      call.Level.Role(),
		Attempt:       call.Ordinal,
		Priority:      t.Priority,
		IncidentRef:   t.caseRef(),
		Title:         t.Title,
		RecipientName: call.Recipient.Name,
		Instruction:   t.instruction(false),
		Rule:          t.Routing.Rule(),
		PortalURL:     portal,
		Elapsed:       elapsedSince(t.At, call.At),
		NextRung:      nextRung,
		NextIn:        nextIn,
		// One thread per incident, so a space shows an escalation unfolding
		// in one conversation instead of scattering its rungs through
		// everything else being posted. The incident id is the natural key:
		// stable, unique, and already the identity every other part of the
		// ladder is filed under.
		ThreadKey: "incident-escalation-" + t.IncidentID,
	}
	if err := n.chat.SendEscalationAlert(ctx, alert); err != nil {
		return Delivery{Channel: ChannelChat}, err
	}
	return Delivery{Channel: ChannelChat, Status: "posted"}, nil
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
