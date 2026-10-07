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

package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxGithubDeliveryBody caps the forwarded envelope. It is 3 MiB against the
// public component's own 2 MiB cap on GitHub's body, leaving room for the
// {id, event, payload} wrapper and for JSON escaping to inflate the payload
// on re-encode. Deliberately NOT the generic 1 MiB default, which a valid
// large delivery would exceed.
const maxGithubDeliveryBody = int64(3 << 20)

// GithubDeliveryHandler applies a GitHub delivery that something else has
// already authenticated.
//
// *** THE HMAC CHECK IS NOT HERE ANY MORE, AND THAT IS THE POINT. ***
// GitHub has to reach the webhook from the internet, and entity-service is
// Organization-visible in Choreo -- exposing it so one route could be
// reached would publish every other route with it. So the public endpoint
// moved to operations/csm-webhooks/github, which verifies the signature over
// the raw body and forwards the result here.
//
// This endpoint is therefore INTERNAL-CLIENT ONLY. It trusts its caller, so
// anything able to reach it could apply an arbitrary delivery -- the client
// credential is what stands in for the signature across that hop.
type GithubDeliveryHandler struct {
	svc service.GithubSyncService
	// internalClientIDs is config.Config.M2MClientIDs.
	internalClientIDs map[string]bool
}

// NewGithubDeliveryHandler constructs the internal delivery endpoint.
func NewGithubDeliveryHandler(svc service.GithubSyncService, internalClientIDs map[string]bool) *GithubDeliveryHandler {
	return &GithubDeliveryHandler{svc: svc, internalClientIDs: internalClientIDs}
}

// githubDeliveryRequest is what the webhook component forwards: the two
// headers that identify a delivery, plus GitHub's body verbatim.
type githubDeliveryRequest struct {
	ID      string          `json:"id"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// Handle handles POST /github/deliveries.
func (h *GithubDeliveryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	id := auth.IdentityFromContext(r.Context())
	if !id.Validated || id.ClientID == "" || !h.internalClientIDs[id.ClientID] {
		apierror.WriteJSON(w, http.StatusUnauthorized,
			"an authorized internal client credential is required")
		return
	}

	var req githubDeliveryRequest
	// A LARGER CAP THAN THE DEFAULT, DELIBERATELY. The public webhook
	// accepts a GitHub body up to 2 MiB, then wraps it in
	// an {id, event, payload} envelope before forwarding -- so a legitimate
	// delivery just under that cap arrives here larger than decodeRequest's
	// 1 MiB default. It would be rejected with a 400 that the public
	// component could only report as a forwarding failure, with nothing
	// anywhere naming the size as the cause.
	if !decodeRequestWithLimit(w, r, &req, maxGithubDeliveryBody, "delivery payload too large") {
		return
	}
	if req.ID == "" || req.Event == "" {
		apierror.WriteJSON(w, http.StatusBadRequest, "id and event are required")
		return
	}

	var payload service.IssuePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "malformed webhook payload")
		return
	}

	// WithSystemIdentity: this request carries an internal CLIENT credential,
	// not a user token, so callerIdentityMiddleware attaches no caller
	// identity -- and githubSyncSvc's repos are Scoped-wrapped, requiring
	// SOME identity on ctx for every write regardless of table. Without this
	// every delivery fails with ErrNoCallerIdentity and the sync is dead.
	//
	// The public endpoint this moved from needed the same stamp for the same
	// reason: it had no user token either, because GitHub cannot present one.
	// Carried across deliberately rather than rediscovered.
	outcome, err := h.svc.HandleWebhook(repository.WithSystemIdentity(r.Context()), service.Delivery{
		ID: req.ID, Event: req.Event, Payload: payload,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDeliverySeen) {
			// 409 rather than 200: the caller maps it back to the 200 GitHub
			// needs, and a distinct status lets it tell "already applied"
			// from "applied just now" without parsing a message.
			apierror.WriteJSON(w, http.StatusConflict, "duplicate delivery, already processed")
			return
		}
		slog.ErrorContext(r.Context(), "github: delivery handling failed",
			"delivery", req.ID, "event", req.Event, "err", err)
		apierror.WriteJSON(w, http.StatusInternalServerError, "could not process the delivery")
		return
	}

	slog.InfoContext(r.Context(), "github: delivery handled",
		"delivery", req.ID, "event", req.Event,
		"action", outcome.Action, "skipped", outcome.Skipped,
		"changeRequestId", outcome.ChangeRequestID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"action":  outcome.Action,
		"skipped": outcome.Skipped,
	})
}
