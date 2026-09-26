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
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxWebhookBody caps what we will read. GitHub's own limit is 25MB; an issue
// body plus labels is orders of magnitude smaller, and an unbounded read from
// an unauthenticated endpoint is a denial-of-service waiting to happen.
const maxWebhookBody = 2 << 20 // 2 MiB

// GithubWebhookHandler receives GitHub issue webhooks.
//
// THIS ENDPOINT IS UNAUTHENTICATED IN THE USUAL SENSE. GitHub cannot present a
// bearer token, so the HMAC signature over the body IS the authentication.
// Nothing is read from the payload before that signature verifies.
type GithubWebhookHandler struct {
	svc    service.GithubSyncService
	secret string
}

// NewGithubWebhookHandler constructs the webhook endpoint.
func NewGithubWebhookHandler(svc service.GithubSyncService, secret string) *GithubWebhookHandler {
	return &GithubWebhookHandler{svc: svc, secret: secret}
}

// Handle handles POST /webhooks/github.
func (h *GithubWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "could not read the request body")
		return
	}

	// Signature first, before the body is parsed or any field is trusted.
	if err := github.VerifySignature(h.secret, body, r.Header.Get(github.SignatureHeader)); err != nil {
		// One response for every failure. The caller learns that it failed and
		// nothing else -- not which check, and never the expected signature.
		// ServiceNow's equivalent returned the computed HMAC in this response,
		// which made the secret irrelevant.
		slog.WarnContext(r.Context(), "github: webhook signature rejected",
			"delivery", r.Header.Get(github.DeliveryHeader),
			"event", r.Header.Get(github.EventHeader))
		apierror.WriteJSON(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	deliveryID := r.Header.Get(github.DeliveryHeader)
	event := r.Header.Get(github.EventHeader)
	if deliveryID == "" || event == "" {
		apierror.WriteJSON(w, http.StatusBadRequest, "missing delivery or event header")
		return
	}

	var payload service.IssuePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "malformed webhook payload")
		return
	}

	outcome, err := h.svc.HandleWebhook(r.Context(), service.Delivery{
		ID: deliveryID, Event: event, Payload: payload,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDeliverySeen) {
			// A redelivery of something already applied. 200, so GitHub stops
			// retrying -- it succeeded, just not this time.
			writeWebhookOK(w, "duplicate delivery, already processed", "")
			return
		}
		// 500 so GitHub retries. The delivery claim is released on failure,
		// so the retry is free to run.
		slog.ErrorContext(r.Context(), "github: webhook handling failed",
			"delivery", deliveryID, "event", event, "err", err)
		apierror.WriteJSON(w, http.StatusInternalServerError, "could not process the webhook")
		return
	}

	slog.InfoContext(r.Context(), "github: webhook handled",
		"delivery", deliveryID, "event", event,
		"action", outcome.Action, "skipped", outcome.Skipped,
		"changeRequestId", outcome.ChangeRequestID)

	writeWebhookOK(w, outcome.Skipped, outcome.Action)
}

func writeWebhookOK(w http.ResponseWriter, skipped, action string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"action":  action,
		"skipped": skipped,
	})
}
