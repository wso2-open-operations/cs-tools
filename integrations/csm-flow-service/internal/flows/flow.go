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

// Package flows holds the hand-ported ServiceNow flows, one Go type per flow
// (or per collapsed family), plus the Registry that routes each consumed event
// to the flows that match it.
//
// This is the pragmatic-hand-port shape (docs/cutover-porting-plan.md §2): the
// configurable engine (spec tree, compiler, spec DB tables) is deferred, so a
// flow is Go code implementing Flow, not a row of JSON. The condition
// evaluator (internal/eval) and the ServiceNow-query translator
// (internal/porting/snquery) remain available — a flow may express its trigger
// condition as a spec.Condition and evaluate it with eval, or just check
// fields directly in Match; both are fine.
//
// Every ported flow's real trigger, condition and actions are recorded in
// docs/flow-porting-specs.md — that is the source of truth a port is written
// from and verified against.
package flows

import (
	"context"
	"sort"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// Recipients resolves who a notification goes to, by reading the CSM database
// directly — the same reads the ServiceNow flows did with GlideRecord against
// sys_user_grmember and project_contact, which are replicated here as
// team_member and project_contact.
//
// Deliberately NOT an entity-service HTTP client: those tables sit in the same
// database as the records that trigger the flows, so going over HTTP would mean
// inventing endpoints to wrap a join and paying a round trip for data one query
// away. *store.Store satisfies this.
type Recipients interface {
	// GroupMemberEmails returns the addresses of everyone in a named team,
	// e.g. "CAB Approval".
	GroupMemberEmails(ctx context.Context, teamName string) ([]string, error)
	// ProjectContactEmails returns a customer project's contact addresses.
	ProjectContactEmails(ctx context.Context, projectID string) ([]string, error)
}

// ChangeRequestDetails is what a change-request notice needs beyond the row
// change that triggered it.
//
// It exists because the outbox snapshot is NOT enough. The trigger records
// to_jsonb(NEW) of the table that changed, so a change_request row change
// carries exactly change_request's own columns — id, state, git_reference, in
// the database's snake_case. The number lives on work_item, the project and
// the requester are two joins away, and none of them appear under the
// camelCase names a JSON payload would suggest. A flow reading them off the
// snapshot silently gets "" for every one, which is how a notice ends up with
// an empty subject and a customer branch that resolves nobody.
type ChangeRequestDetails struct {
	// Number is the human-readable reference, e.g. "CHG0031234" — work_item.number.
	Number string
	// GitReference is what the internal subflow branched on to pick a team.
	GitReference string
	// RequesterName is who opened the change request.
	RequesterName string
	// ProjectID is the project the change request belongs to. The customer
	// portal nests its change-request page under the project, so a notice
	// without this cannot be linked.
	ProjectID string
	// ProjectName is shown in the email body.
	ProjectName string
	// ShortDescription is work_item.subject -- ServiceNow's "Short
	// description", shown as the second body line of a plan-date notice.
	ShortDescription string
	// Description is the change request's full description, shown beneath it.
	Description string
	// ActorName is whoever last changed the record, rendered LAST NAME FIRST
	// because the ServiceNow templates interpolate the two pills in that order.
	ActorName string
	// ActorIsWSO2 reports whether that person has a wso2.com address. The
	// internal plan-date notice fires only when a CUSTOMER moved the date --
	// the original's trigger says sys_updated_byNOT LIKE@wso2.com -- so the
	// port needs the same distinction.
	ActorIsWSO2 bool
}

// ChangeRequests reads a change request's surrounding detail. Same reasoning as
// Recipients: the rows are in the database this service already holds a pool
// for, so a join beats inventing an endpoint to wrap one. *store.Store
// satisfies this.
type ChangeRequests interface {
	ChangeRequestDetails(ctx context.Context, id string) (ChangeRequestDetails, error)
}

// EventPublisher is the bus write a flow makes: one record, keyed so every
// event about the same entity stays ordered on one partition.
type EventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// Deps are the shared clients a flow may use. A flow uses only what it needs;
// any of these may be nil in a deployment that hasn't configured it, so a flow
// that dereferences one is responsible for the entity being configured (or for
// letting the call fail cleanly).
type Deps struct {
	// Recipients resolves notification audiences from the CSM database.
	// An interface so a flow's Run is testable with a fake — Run is where a
	// port's real behaviour lives, and a concrete type here would leave it
	// exercisable only against a live database.
	Recipients Recipients
	// ChangeRequests reads the detail a change-request notice needs that the
	// triggering row change does not carry. An interface for the same reason
	// as Recipients.
	ChangeRequests ChangeRequests
	// Producer publishes back onto the bus: a notification-request event that
	// csm-notification-service sends, or (later) timer.fired from the sweeper.
	// An interface for the same reason as Entity; *eventbus.Producer satisfies it.
	Producer EventPublisher
	// EmailDebugRecipients, when non-empty, replaces the real audience of every
	// notification a flow requests — approval groups, project contacts,
	// watchers — so a dev or staging deployment can be exercised without mail
	// reaching real people. See config.Config.EmailDebugRecipients.
	//
	// A flow honouring this must still RESOLVE its real recipients first and
	// swap only the final list: that keeps a broken entity-service lookup
	// visible instead of masked, and means a flow with no real audience still
	// sends nothing rather than mailing the debug list about an event nobody
	// would have been told about.
	EmailDebugRecipients []string
}

// Event is a decoded bus record handed to a flow.
type Event struct {
	// Envelope is the parsed wire envelope: Type, EntityID, raw Payload.
	Envelope events.Envelope
	// Record is the underlying bus record, for idempotency coordinates and
	// the raw bytes.
	Record eventbus.Record
}

// Flow is one ported ServiceNow flow.
//
//   - Key is a stable identifier (e.g. "watch_list"). It names the flow in
//     logs and is half of the (event, flow) idempotency key once the dispatch
//     log lands (docs/architecture.md §9). Renaming one is a semantic change.
//   - Match is the trigger + condition gate. It MUST be pure (no I/O): it
//     decides, from the event alone, whether this flow reacts. Keeping it pure
//     is what makes the routing cheap and exhaustively testable.
//   - Run performs the flow's actions. A non-nil error causes the record to be
//     retried (eventbus.handleAttempts), so Run must be safe to re-run — see
//     the idempotency note on Registry.Handle.
type Flow interface {
	Key() string
	Match(evt Event) bool
	Run(ctx context.Context, evt Event, deps Deps) error
}

// The production producer satisfies EventPublisher. *store.Store satisfies
// Recipients, asserted in cmd/consumer where the two are wired — asserting it
// here would make this package import the store, which it has no other reason
// to know about.
var _ EventPublisher = (*eventbus.Producer)(nil)

// EntityTriggered is implemented by a flow whose trigger is a row change
// rather than a bus event. Optional: a flow that only reacts to typed bus
// events (case.created and friends) does not implement it and is simply not
// counted when the drainer decides what to poll for.
//
// Declaring it on the flow, rather than configuring it in main, keeps the
// trigger source next to the Match that depends on it — registering a flow
// cannot then silently fail to drain the table it needs.
type EntityTriggered interface {
	// TriggerEntityTypes are the entity_type values in event_outbox this flow
	// can match, e.g. "change_request".
	TriggerEntityTypes() []string
}

// TriggerEntityTypes is the union of every registered flow's trigger entity
// types, deduplicated and ordered so the drainer's query plan and its logs are
// stable between restarts. Empty when no registered flow is row-triggered, in
// which case the drainer has nothing to poll for.
func TriggerEntityTypes(fs []Flow) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		t, ok := f.(EntityTriggered)
		if !ok {
			continue
		}
		for _, et := range t.TriggerEntityTypes() {
			if et == "" || seen[et] {
				continue
			}
			seen[et] = true
			out = append(out, et)
		}
	}
	sort.Strings(out)
	return out
}
