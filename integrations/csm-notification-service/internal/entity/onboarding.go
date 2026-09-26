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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OnboardingStep names the two onboarding_step rows this service writes
// for a membership — entity-service's OnboardingStepName enum also has
// DATABASE (written in-process by its own Salesforce ingest) and
// REGISTRATION (written by the customer portal backend), neither of which
// this service ever touches.
type OnboardingStep string

const (
	// OnboardingStepIdentity is the Asgardeo-identity step
	// (internal/scim.Client.EnsureExternalUser).
	OnboardingStepIdentity OnboardingStep = "IDENTITY"
	// OnboardingStepEmail is the invitation-email step.
	OnboardingStepEmail OnboardingStep = "EMAIL"
)

// OnboardingStepStatus mirrors entity-service's OnboardingStepStatus enum.
type OnboardingStepStatus string

const (
	OnboardingStepSucceeded OnboardingStepStatus = "SUCCEEDED"
	OnboardingStepFailed    OnboardingStepStatus = "FAILED"
	OnboardingStepSkipped   OnboardingStepStatus = "SKIPPED"
)

// OnboardingStepRequest is the body of PUT
// /onboarding-steps/{membershipSfId}/{step} — mirrors entity-service's
// domain.UpsertOnboardingStepRequest, minus the fields (projectId,
// projectContactId) this service has no value for. MembershipSfID and Step
// go in the path, not the body.
type OnboardingStepRequest struct {
	MembershipSfID string               `json:"-"`
	Step           OnboardingStep       `json:"-"`
	Status         OnboardingStepStatus `json:"status"`
	// LastError is only kept by entity-service when Status is FAILED (a
	// stale error must not outlive a success) and truncated there to 1000
	// characters; omitted from the body when empty.
	LastError string `json:"lastError,omitempty"`
	// EventType is the event this write is a reaction to —
	// events.TypeProjectContactInvited for every write this service makes.
	EventType string `json:"eventType"`
	// EventModifiedOn is the version this write is based on, and it is the
	// wire contract with entity-service's upsert rule: a write only moves a
	// row's status/lastError/eventType when its eventModifiedOn is **not
	// older** than the stored one. Not-older, not newer — equal timestamps
	// land, which is what lets a retry of the same version rewrite its own
	// row.
	//
	// Send the Salesforce LastModifiedDate the event carries, never
	// time.Now(): the timestamp identifies the membership version this
	// outcome belongs to, so a delayed delivery of an older invitation is
	// correctly rejected by entity-service instead of overwriting a newer
	// result. dispatch.Dispatcher.recordOnboardingStep falls back to the
	// processing time only when the payload carries no timestamp at all.
	EventModifiedOn time.Time `json:"eventModifiedOn"`
	Email           string    `json:"email"`
	ContactSfID     string    `json:"contactSfId,omitempty"`
}

// RecordedOnboardingStep is one row of GET
// /onboarding-steps/{membershipSfId}. Only the two fields the caller acts
// on are decoded; entity-service returns more.
type RecordedOnboardingStep struct {
	Step   OnboardingStep       `json:"step"`
	Status OnboardingStepStatus `json:"status"`
}

// getOnboardingStepsResponse is the body of GET
// /onboarding-steps/{membershipSfId}.
type getOnboardingStepsResponse struct {
	Steps []RecordedOnboardingStep `json:"steps"`
}

// EmailAlreadySent reports whether the ledger already holds a SUCCEEDED
// EMAIL step for the membership -- that is, whether an invitation has
// already gone out for it, by this service on an earlier delivery or by
// whoever onboarded the contact.
//
// This is the one durable guard against sending a second invitation. The
// in-process claim in dispatch only covers one process's lifetime, and the
// ingest's own duplicate guard only covers the case where the Salesforce
// version is unchanged; neither survives a redelivery to a different
// replica, and neither knows about a membership that was onboarded
// synchronously through the customer portal and only later re-ingested from
// a Salesforce event. The ledger is a row in Postgres that both services
// share, so it does.
//
// An unknown membership yields false with no error: entity-service answers
// an empty list rather than a 404.
func (c *CustomerEntityClient) EmailAlreadySent(ctx context.Context, membershipSfID string) (bool, error) {
	if membershipSfID == "" {
		return false, fmt.Errorf("entity: membershipSfId is required to read onboarding steps")
	}
	body, err := c.do(ctx, http.MethodGet, "/onboarding-steps/"+url.PathEscape(membershipSfID), nil)
	if err != nil {
		return false, err
	}
	var out getOnboardingStepsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("entity: decode onboarding steps response: %w", err)
	}
	for _, s := range out.Steps {
		if s.Step == OnboardingStepEmail && s.Status == OnboardingStepSucceeded {
			return true, nil
		}
	}
	return false, nil
}

// RecordOnboardingStep calls PUT /onboarding-steps/{membershipSfId}/{step}
// — the one write this otherwise read-only client makes — recording the
// latest outcome of one onboarding step for one membership. Idempotent on
// entity-service's side: a repeat for the same (membership, step) rewrites
// the row and bumps its attemptCount. The response (the resulting row) is
// discarded; nothing here needs it.
//
// entity-service restricts this endpoint to internal callers (its
// AUTH_INTERNAL_CLIENT_IDS) — a 403 here means this service's OAuth2 client
// id isn't in that list, not a bad request.
func (c *CustomerEntityClient) RecordOnboardingStep(ctx context.Context, req OnboardingStepRequest) error {
	if req.MembershipSfID == "" || req.Step == "" {
		return fmt.Errorf("entity: membershipSfId and step are required to record an onboarding step")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("entity: encode RecordOnboardingStep request: %w", err)
	}
	path := "/onboarding-steps/" + url.PathEscape(req.MembershipSfID) + "/" + url.PathEscape(string(req.Step))
	if _, err := c.do(ctx, http.MethodPut, path, body); err != nil {
		return err
	}
	return nil
}
