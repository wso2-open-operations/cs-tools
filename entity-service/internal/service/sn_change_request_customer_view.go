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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// This file is the ServiceNow data source's half of "which change requests a
// customer sees".
//
// On the Postgres data source the rule is the SQL fragment in
// repository/change_request_visibility.go: a customer sees a change request
// designated to them (in whatever state it is in) or a LEGACY one, and a legacy
// one is visible in every state except New, Assess and Authorize.
//
// The ServiceNow data source has no designation: a change request there is asked
// of nobody through this service, ServiceNow runs the approvals itself, so every
// change request on it is legacy by definition and the rule reduces to the state
// list. ServiceNow's own search does not apply that list for us: it returns every
// state a caller names and, for a caller who names none, every state there is (the
// customer portals have always hidden New, Assess and Authorize by never ASKING
// for them, which is no guarantee for a caller that asks differently). So this
// service applies the legacy rule itself, for every caller that is not WSO2 staff:
//
//   - a search or aggregate is narrowed to the visible states before it is sent
//     (none named means "every visible state", never "no state filter"), and one
//     that names only hidden states is answered "none" without a request;
//   - what comes back is checked again, and a change request in a hidden (or
//     unknown) state is dropped and logged;
//   - the project's state vocabulary leaves the three hidden states out.
//
// Who is staff is not decided here. callerIdentityMiddleware resolves every
// request's scope once (AccessService.ResolveScope: the CSM portal's backend for a
// user of WSO2's domain, the machine-to-machine clients, an internal user) and
// attaches it to the context; a request whose scope is Unrestricted is staff, and
// every other request is a customer, including one for which no scope could be
// resolved (no identity, an unknown client, a customer portal client with no
// database to look the user up in). That is the same line the Postgres rule draws
// (crViewer), and it fails closed: the customer view only ever narrows.
//
// What this does NOT change: the Postgres data source (which has the stricter,
// per-customer rule and keeps a designated change request in Authorize visible),
// staff, and ServiceNow's by-id reads and writes (detail, approvals, decision,
// PATCH, comments), which stay ServiceNow's own decision. The three counters of the
// change request stats (total, outstanding, active) are computed by ServiceNow over
// every state and are not corrected here.

// customerVisibleChangeRequestStates is the legacy visibility rule's state list:
// everything past Authorize. It is repository.crLegacyVisibleStates in this
// package's vocabulary, and TestCustomerVisibleChangeRequestStates pins the
// ServiceNow keys it maps to.
var customerVisibleChangeRequestStates = []domain.ChangeRequestState{
	domain.ChangeRequestStateCustomerApproval,
	domain.ChangeRequestStateScheduled,
	domain.ChangeRequestStateImplement,
	domain.ChangeRequestStateReview,
	domain.ChangeRequestStateCustomerReview,
	domain.ChangeRequestStateRollback,
	domain.ChangeRequestStateClosed,
	domain.ChangeRequestStateCanceled,
}

// isCustomerVisibleChangeRequestState reports whether a customer may see a change
// request in this state. A state this service has no word for is not visible: the
// rule is a list of what may be shown, not of what may not.
func isCustomerVisibleChangeRequestState(s domain.ChangeRequestState) bool {
	for _, v := range customerVisibleChangeRequestStates {
		if v == s {
			return true
		}
	}
	return false
}

// isCustomerHiddenChangeRequestStateItem reports whether a state choice of the
// project's vocabulary (ServiceNow's {id, label}) is New, Assess or Authorize,
// matched by ServiceNow's numeric key or by the label, whichever the choice
// carries.
func isCustomerHiddenChangeRequestStateItem(item domain.ChoiceListItem) bool {
	switch strings.TrimSpace(item.ID) {
	case "-5", "-4", "-3":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(item.Label)) {
	case "new", "assess", "authorize":
		return true
	}
	return false
}

// customerViewApplies reports whether the customer's view of change requests
// applies to ctx: true unless the request's resolved scope is unrestricted (WSO2
// staff). A request with no resolved scope is a customer's: fail closed.
func customerViewApplies(ctx context.Context) bool {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	return !ok || !scope.Unrestricted
}

// customerChangeRequestStates is the state list a customer's search is sent with.
//
// requested is what the caller named. The result is those of them a customer may
// see; none named is every state a customer may see (never "no filter", which
// ServiceNow reads as every state there is). none is true when states were named
// and every one of them is hidden: the answer is then "no change requests" and
// nothing needs asking.
func customerChangeRequestStates(requested []domain.ChangeRequestState) (states []domain.ChangeRequestState, none bool) {
	if len(requested) == 0 {
		return append([]domain.ChangeRequestState(nil), customerVisibleChangeRequestStates...), false
	}
	for _, s := range requested {
		if isCustomerVisibleChangeRequestState(s) {
			states = append(states, s)
		}
	}
	return states, len(states) == 0
}

// dropHiddenChangeRequests removes, from a search page ServiceNow answered for a
// customer, every change request whose state a customer may not see, and says how
// many it removed. ServiceNow was asked for the visible states only, so this is
// expected to remove nothing; if it does, ServiceNow did not honour the state
// list and the count is logged (a number, never a change request).
func dropHiddenChangeRequests(ctx context.Context, views []domain.SearchChangeRequestView) (kept []domain.SearchChangeRequestView, dropped int) {
	kept = make([]domain.SearchChangeRequestView, 0, len(views))
	for _, v := range views {
		if v.State != nil && isCustomerVisibleChangeRequestState(domain.ChangeRequestState(*v.State)) {
			kept = append(kept, v)
			continue
		}
		dropped++
	}
	if dropped > 0 {
		slog.WarnContext(ctx, "servicenow returned change requests in a state hidden from customers; dropped them", "dropped", dropped)
	}
	return kept, dropped
}

// dropHiddenStateBuckets is dropHiddenChangeRequests for a customer's aggregate
// by state: a bucket of a state a customer may not see (or does not know) is
// removed, and the total is reduced by what it held.
func dropHiddenStateBuckets(ctx context.Context, resp domain.AggregateResponse) domain.AggregateResponse {
	kept := make([]domain.AggregateBucket, 0, len(resp.Groups))
	dropped := 0
	for _, b := range resp.Groups {
		if isCustomerVisibleChangeRequestState(domain.ChangeRequestState(b.Key)) {
			kept = append(kept, b)
			continue
		}
		dropped += b.Count
	}
	if len(kept) != len(resp.Groups) {
		slog.WarnContext(ctx, "servicenow returned change request state buckets hidden from customers; dropped them", "dropped", dropped)
		resp.Groups = kept
		resp.TotalRecords = max(resp.TotalRecords-dropped, 0)
	}
	return resp
}
