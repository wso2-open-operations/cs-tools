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

// Command server is the public GitHub webhook endpoint for the CSM platform.
//
// *** IT EXISTS BECAUSE entity-service CANNOT BE PUBLIC. *** GitHub has to
// reach this endpoint from the internet; entity-service is
// Organization-visible in Choreo, and making one of its routes reachable
// would publish every other route with it. So the webhook moved out here and
// entity-service kept the part that touches data.
//
// This process holds NO database credentials and makes NO decisions about
// what a delivery means. It does exactly four things:
//
//  1. read the body, capped
//  2. verify GitHub's HMAC over those exact bytes
//  3. take the delivery id and event type from their headers
//  4. forward all three to entity-service as an internal client
//
// *** THE BODY IS FORWARDED BYTE FOR BYTE. *** The signature is computed over
// the raw bytes, so re-encoding the JSON -- even just reordering keys --
// would make it unverifiable downstream and turn a correct delivery into a
// mystery. It is read once and passed on unchanged.
package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-webhooks/github/internal/entity"
	"github.com/wso2-open-operations/cs-tools/operations/csm-webhooks/github/internal/webhook"
)

// maxWebhookBody caps what will be read. GitHub's own limit is 25MB; an issue
// body plus labels is orders of magnitude smaller, and an unbounded read from
// an endpoint that anyone can reach is a denial of service waiting to happen.
const maxWebhookBody = 2 << 20 // 2 MiB

func main() {
	secret := mustEnv("GITHUB_WEBHOOK_SECRET")
	client := entity.NewClient(entity.Config{
		BaseURL:      strings.TrimRight(mustEnv("ENTITY_BASE_URL"), "/"),
		TokenURL:     mustEnv("ENTITY_TOKEN_URL"),
		ClientID:     mustEnv("ENTITY_CLIENT_ID"),
		ClientSecret: mustEnv("ENTITY_CLIENT_SECRET"),
		Scopes:       splitComma(os.Getenv("ENTITY_SCOPES")),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhooks/github", handle(secret, client))
	// Unauthenticated liveness, so Choreo can probe without a credential.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("CSM GitHub Webhook started", "addr", ":"+port)
	// Every timeout is set, not just the header one. ReadHeaderTimeout
	// bounds the headers alone: a client that sends valid headers and then
	// dribbles the body a byte at a time holds a goroutine open forever,
	// and the 2 MiB cap does not help -- it limits how much arrives, never
	// how long it takes. Choreo's gateway does not promise a deadline or
	// request buffering either, so the bound has to live here.
	//
	// WriteTimeout exceeds the entity-service client's own 30s timeout on
	// purpose: the forward has to be allowed to fail on its own terms and
	// return a 500 GitHub can retry, rather than having the response
	// connection cut out from under it first.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func handle(secret string, client *entity.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// MaxBytesReader, not io.LimitReader. A LimitReader stops at the cap
		// and reports success, so an oversized delivery would arrive here
		// truncated, fail the HMAC over those partial bytes, and be logged
		// as "webhook signature rejected" -- the one message guaranteed to
		// send whoever reads it looking at the secret instead of the size.
		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				slog.Warn("webhook body too large",
					"delivery", r.Header.Get(webhook.DeliveryHeader),
					"event", r.Header.Get(webhook.EventHeader),
					"limit", maxWebhookBody)
				writeJSON(w, http.StatusRequestEntityTooLarge, `{"message":"request body too large"}`)
				return
			}
			writeJSON(w, http.StatusBadRequest, `{"message":"could not read the request body"}`)
			return
		}

		// Signature first, before anything in the payload is trusted or even
		// parsed.
		if err := webhook.VerifySignature(secret, body, r.Header.Get(webhook.SignatureHeader)); err != nil {
			// One response for every failure. The caller learns that it
			// failed and nothing else -- not which check, and never the
			// expected signature.
			slog.Warn("webhook signature rejected",
				"delivery", r.Header.Get(webhook.DeliveryHeader),
				"event", r.Header.Get(webhook.EventHeader))
			writeJSON(w, http.StatusUnauthorized, `{"message":"invalid signature"}`)
			return
		}

		deliveryID := r.Header.Get(webhook.DeliveryHeader)
		event := r.Header.Get(webhook.EventHeader)
		if deliveryID == "" || event == "" {
			writeJSON(w, http.StatusBadRequest, `{"message":"missing delivery or event header"}`)
			return
		}

		out, err := client.Deliver(r.Context(), entity.Delivery{
			ID: deliveryID, Event: event, Payload: body,
		})
		switch {
		case errors.Is(err, entity.ErrDuplicate):
			// 200, so GitHub stops retrying: it succeeded, just not this time.
			slog.Info("duplicate delivery", "delivery", deliveryID, "event", event)
			writeJSON(w, http.StatusOK, `{"status":"ok","skipped":"duplicate delivery, already processed"}`)
		case err != nil:
			// 500 so GitHub retries. A delivery lost here is lost for good:
			// nothing else re-reads GitHub's event stream.
			slog.Error("forward failed", "delivery", deliveryID, "event", event, "err", err)
			writeJSON(w, http.StatusInternalServerError, `{"message":"could not process the webhook"}`)
		default:
			slog.Info("delivery forwarded", "delivery", deliveryID, "event", event)
			writeJSON(w, http.StatusOK, string(out))
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func splitComma(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
