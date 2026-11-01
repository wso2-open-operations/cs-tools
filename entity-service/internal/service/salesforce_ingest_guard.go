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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// shouldSkipIngest is the duplicate-event guard every non-membership ingest
// runs before fetching and writing a record; it is ingestMembership's guard
// (which reads onboarding_step) rebuilt on the salesforce_ingest_state
// ledger. Salesforce emits several UPDATED events per save and Service Bus
// redelivers, so the same record version arrives more than once. The rules:
//
//  1. skip when the ledger holds a SUCCEEDED row whose event_modified_on is
//     not before the incoming version — that version (or a later one) is
//     already written;
//  2. a missing or unparseable lastModified skips the guard, not the event:
//     the caller's upsert is idempotent, so running it again is harmless,
//     and eventModifiedOn is then the current time so the ledger still
//     records a version that orders after everything before it;
//  3. a row last written by a DELETED event never blocks: an undelete
//     (RESTORED) keeps the record's LastModifiedDate, and the row must be
//     allowed to leave its deleted state.
//
// A FAILED row never blocks either — a retry of a failed version is the
// point. eventModifiedOn is what the caller records in the ledger after its
// write, whether or not the guard ran.
func shouldSkipIngest(ctx context.Context, states repository.SalesforceIngestStateRepository, entity, sfID, eventType string, lastModified *string) (skip bool, eventModifiedOn time.Time, err error) {
	eventModifiedOn, hasModified := parseSalesforceLastModified(lastModified)
	if isResyncApply(ctx) {
		// The admin resync overwrites on purpose: the record version is
		// still what the ledger records, but no stored version blocks it.
		if !hasModified {
			eventModifiedOn = time.Now().UTC()
		}
		return false, eventModifiedOn, nil
	}
	if !hasModified {
		slog.WarnContext(ctx, "salesforce: record has no parseable lastModifiedDate, skipping duplicate guard",
			"entity", entity, "sfId", sfID, "eventType", eventType, "lastModifiedDate", derefString(lastModified))
		return false, time.Now().UTC(), nil
	}
	st, err := states.Get(ctx, entity, sfID)
	if err != nil {
		return false, eventModifiedOn, err
	}
	if st != nil && st.Status == domain.SalesforceIngestSucceeded &&
		st.EventType != domain.SalesforceEventDeleted && !st.EventModifiedOn.Before(eventModifiedOn) {
		slog.InfoContext(ctx, "salesforce: record version already ingested, skipping",
			"entity", entity, "sfId", sfID, "eventType", eventType, "eventModifiedOn", eventModifiedOn, "recordedOn", st.EventModifiedOn)
		return true, eventModifiedOn, nil
	}
	return false, eventModifiedOn, nil
}

// resyncApplyKey marks a context as the admin resync's apply path.
type resyncApplyKey struct{}

// withResyncApply returns ctx marked as the admin resync's apply path. It is
// set by the resync service only; the event path never sets it, so every
// ingest behaves as before there. On a marked context the duplicate-event
// guard (shouldSkipIngest and the membership ingest's own onboarding-step
// guard) does not skip, the ingests only add and update (no row is
// deleted), no invitation or welcome event is published, and no partner
// refresh is started behind a membership.
func withResyncApply(ctx context.Context) context.Context {
	return context.WithValue(ctx, resyncApplyKey{}, true)
}

func isResyncApply(ctx context.Context) bool {
	v, _ := ctx.Value(resyncApplyKey{}).(bool)
	return v
}
