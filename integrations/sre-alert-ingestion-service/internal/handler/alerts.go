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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/alertpayload"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/idgen"
	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/severity"
)

// alertStore is the subset of internal/store.Store AlertHandler depends on.
type alertStore interface {
	Enqueue(ctx context.Context, id string, buildPayload func(alertNumber string) ([]byte, error)) (alertNumber string, err error)
}

// AlertRequest is the inbound, vendor-agnostic normalized alert payload
// third-party monitoring/alerting tools POST to this service. It is
// intentionally decoupled from CreateIncidentRequest's shape — a vendor
// speaks "alerts", CSM speaks "incidents"; MapToIncident bridges the two.
type AlertRequest struct {
	Source           string `json:"source"`
	Severity         string `json:"severity"`
	Service          string `json:"service"`
	MetricName       string `json:"metricName"`
	Category         string `json:"category,omitempty"`
	Environment      string `json:"environment,omitempty"`
	UniqueIdentifier string `json:"uniqueIdentifier,omitempty"`
	Description      string `json:"description"`
}

// validate reports the first missing or malformed field, or "" if req is
// well-formed. Category/Environment/UniqueIdentifier are genuinely optional
// (see severity.MapCategory's safe default and the WorkNotes builder below).
//
// Source, Severity, Service, MetricName, Environment, and UniqueIdentifier
// are each embedded verbatim into a single-line, structurally-meaningful
// context — buildSubject's one Subject line, or one `"Label: %s\n"` line of
// buildWorkNotes — so none may contain "\n"/"\r": a crafted Source of
// "real-source\nAlert identifier: forged-id", for instance, would render as
// if the alert legitimately carried a different, attacker-chosen
// identifier, spoofing engineer-facing metadata the on-call responder reads
// and trusts. Description is deliberately exempt: it maps to
// AdditionalComments, a genuine free-text narrative field, not a
// line-delimited structure a newline could forge a fake entry inside.
//
// Source and UniqueIdentifier are additionally checked against
// csmclient.TagDelimiterChars, since both are also embedded in a dedup/group tag —
// see that constant's own doc comment.
func (req AlertRequest) validate() string {
	for name, v := range map[string]string{
		"source": req.Source, "severity": req.Severity, "service": req.Service,
		"metricName": req.MetricName, "environment": req.Environment, "uniqueIdentifier": req.UniqueIdentifier,
	} {
		if strings.ContainsAny(v, "\n\r") {
			return name + " must not contain a newline"
		}
	}
	switch {
	case strings.TrimSpace(req.Source) == "":
		return "source is required"
	case strings.ContainsAny(req.Source, csmclient.TagDelimiterChars):
		return "source must not contain '[', ']', or ':'"
	case strings.ContainsAny(req.UniqueIdentifier, csmclient.TagDelimiterChars):
		return "uniqueIdentifier must not contain '[', ']', or ':'"
	case strings.TrimSpace(req.Severity) == "":
		return "severity is required"
	case strings.TrimSpace(req.Service) == "":
		return "service is required"
	case strings.TrimSpace(req.MetricName) == "":
		return "metricName is required"
	case strings.TrimSpace(req.Description) == "":
		return "description is required"
	}
	return ""
}

// MapToIncident builds the CreateIncidentRequest this service will
// eventually send to csm-integration-service, from req, the buffered alert
// row's own human-readable alert number, and the configured callerID (see
// AlertHandler.callerID's doc comment — CSM has no "system" user concept for
// machine-created incidents today, so this must be a real,
// operator-provisioned CSM user id passed in via config, never guessed or
// hardcoded here).
//
// alertNumber is embedded into the built Subject as a dedup tag
// (csmclient.DedupTag) — see buildSubject's doc comment for why.
func MapToIncident(req AlertRequest, alertNumber, callerID string) csmclient.CreateIncidentRequest {
	iu := severity.MapImpactUrgency(req.Severity)
	category := severity.MapCategory(req.Category)

	out := csmclient.CreateIncidentRequest{
		CallerID:  callerID,
		Category:  category,
		ServiceID: req.Service,
		Impact:    iu.Impact,
		Urgency:   iu.Urgency,
		Subject:   buildSubject(alertNumber, req),
	}

	if ct, ok := severity.MapContactType(req.Source); ok {
		out.ContactType = &ct
	}

	// Description is the customer/incident-facing narrative -> AdditionalComments.
	comments := req.Description
	out.AdditionalComments = &comments

	// WorkNotes carries the alert's own metadata for the engineer who picks
	// this up — the fields that don't have a home anywhere else on
	// CreateIncidentRequest (source, environment, unique identifier, the
	// raw severity string as reported).
	if notes := buildWorkNotes(req); notes != "" {
		out.WorkNotes = &notes
	}

	return out
}

// buildSubject composes CreateIncidentRequest.Subject from the buffered
// alert row's own human-readable alert number and the alert's metric name
// and source, e.g. "[alert:ALT0000123] [group:azure:uid-123] [azure]
// high_error_rate alert: svc-checkout".
//
// The leading csmclient.DedupTag(alertNumber) is not cosmetic: it's this
// service's own dedup key for internal/worker's pre-retry
// SearchIncidentByTag check (a failed POST /incidents doesn't prove the
// incident wasn't actually created — the response could have been lost —
// so a retry needs a way to find a previous attempt's incident by something
// this service fully controls). alertNumber is guaranteed unique per
// buffered alert (this service's own Postgres sequence — see
// internal/store.PostgresStore.Enqueue) and is this row's externally-facing
// identifier, unlike AlertRequest.UniqueIdentifier, which is
// vendor-supplied and optional.
//
// When req.UniqueIdentifier is set, csmclient.GroupTag(req.Source,
// req.UniqueIdentifier) is also embedded — deliberately the *same* value
// across every alert reporting this condition, unlike the per-row dedup
// tag — so a later alert for the same condition can find this incident via
// internal/worker.tryGroup's search and attach instead of creating a new
// one. Omitted entirely when there's no UniqueIdentifier to group by.
//
// Full human-readable detail still belongs in AdditionalComments/WorkNotes,
// not here — this stays short and scannable.
func buildSubject(alertNumber string, req AlertRequest) string {
	tags := csmclient.DedupTag(alertNumber)
	if req.UniqueIdentifier != "" {
		tags += " " + csmclient.GroupTag(req.Source, req.UniqueIdentifier)
	}
	return fmt.Sprintf("%s [%s] %s alert: %s", tags, req.Source, req.MetricName, req.Service)
}

// deriveAlertStatus maps an inbound alert onto this service's own
// FIRING/RESOLVED vocabulary for the alert-incident-mapping contract
// (csmclient.CreateAlertIncidentMappingRequest.AlertStatus — see
// internal/worker's incident-grouping logic). AlertRequest carries no
// explicit status/state field from the vendor today — only Severity — so
// this reuses the one signal internal/severity.MapImpactUrgency already
// treats specially: "ok" is the sole severity value documented there as
// meaning the condition has cleared ("a fully-resolved (ok) signal warrants
// neither" elevated impact nor urgency — see severity.go). Every other
// severity value means the condition is still active, hence FIRING.
func deriveAlertStatus(req AlertRequest) string {
	if strings.EqualFold(strings.TrimSpace(req.Severity), "ok") {
		return "RESOLVED"
	}
	return "FIRING"
}

// buildWorkNotes composes an internal-engineer-facing note from the alert
// fields that have no other home on CreateIncidentRequest. Returns "" only
// if req somehow carries none of these (validate() already requires
// Source/MetricName, so this is effectively always non-empty in practice).
func buildWorkNotes(req AlertRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Alert source: %s\n", req.Source)
	fmt.Fprintf(&b, "Reported severity: %s\n", req.Severity)
	fmt.Fprintf(&b, "Metric: %s\n", req.MetricName)
	if req.Environment != "" {
		fmt.Fprintf(&b, "Environment: %s\n", req.Environment)
	}
	if req.UniqueIdentifier != "" {
		fmt.Fprintf(&b, "Alert identifier: %s\n", req.UniqueIdentifier)
	}
	return strings.TrimRight(b.String(), "\n")
}

// AlertHandler handles POST /alerts.
type AlertHandler struct {
	store alertStore
	// callerID is CreateIncidentRequest.CallerID for every incident this
	// service creates: a real CSM user id the operator must provision ahead
	// of time (SRE_ALERT_CALLER_ID). CSM has no "system"/machine-caller
	// concept today. This is required, non-empty config, never a guessed or
	// hardcoded value — see this service's README/CLAUDE.md.
	callerID string
}

// NewAlertHandler creates an AlertHandler. callerID must be non-empty — the
// caller (cmd/server/main.go) is expected to fail startup via mustEnv if
// SRE_ALERT_CALLER_ID is unset, rather than this constructor silently
// accepting an empty string.
func NewAlertHandler(store alertStore, callerID string) *AlertHandler {
	return &AlertHandler{store: store, callerID: callerID}
}

// errValidation wraps a validate() failure message so enqueueAlert's callers
// (CreateAlert and every vendor-adapter handler in adapter_*.go) can tell a
// validation failure (-> 400) apart from a persistence failure (-> 500)
// without enqueueAlert exposing anything more elaborate than a single
// sentinel-wrapped error — this package has exactly two failure modes here,
// not a general-purpose error-classification framework.
var errValidation = errors.New("alert validation failed")

// enqueueAlert is the vendor-agnostic core of alert ingestion: validate,
// generate an id, map to a CreateIncidentRequest, and persist to the
// buffer — never attempting delivery inline. This ordering (persist, then
// return; delivery happens later, on the worker's own schedule) is a
// correctness requirement, not just a latency optimization: if this service
// crashed between a delivery attempt and persisting, an alert could be lost
// with no record it was ever received. Persisting first, unconditionally,
// before any attempt is made, is what makes "never lose a buffered alert,
// even across this service's own restart" true regardless of when in the
// flow a crash happens.
//
// Both CreateAlert (the generic POST /alerts entrypoint) and every
// vendor-adapter handler (adapter_azure.go, adapter_site24x7.go,
// adapter_opensearch.go, adapter_grafana.go) call this same method after
// building their own AlertRequest — from the wire-format JSON directly for
// CreateAlert, from a vendor-native payload for an adapter — so none of
// validation/id-generation/mapping/persistence is ever duplicated.
//
// On failure, err wraps errValidation (via errors.Is) for a validate()
// failure, or is a bare error from h.store.Enqueue for a persistence
// failure — callers map the former to 400 and the latter to 500, matching
// CreateAlert's original behavior.
func (h *AlertHandler) enqueueAlert(ctx context.Context, req AlertRequest) (id, alertNumber string, err error) {
	if msg := req.validate(); msg != "" {
		return "", "", fmt.Errorf("%w: %s", errValidation, msg)
	}

	// id is still generated here (internal/idgen), before persistence: it is
	// alert_buffer's DB primary key, needed up front as internal/store.Store's
	// Enqueue parameter. The row's externally-facing identifier, by contrast,
	// is generated by the store itself (this service's own Postgres sequence)
	// during Enqueue — see buildPayload below and internal/store.Store.Enqueue's
	// doc comment for why that has to be a callback rather than a plain value.
	id = idgen.New()

	alertNumber, err = h.store.Enqueue(ctx, id, func(alertNumber string) ([]byte, error) {
		incidentReq := MapToIncident(req, alertNumber, h.callerID)
		payload := alertpayload.Payload{
			CreateIncidentRequest: incidentReq,
			Source:                req.Source,
			UniqueIdentifier:      req.UniqueIdentifier,
			Service:               req.Service,
			MetricName:            req.MetricName,
			AlertStatus:           deriveAlertStatus(req),
		}
		return json.Marshal(payload)
	})
	if err != nil {
		return "", "", err
	}

	slog.InfoContext(ctx, "alert buffered", "id", id, "alertNumber", alertNumber, "source", req.Source, "severity", req.Severity, "service", req.Service)
	return id, alertNumber, nil
}

// CreateAlert handles POST /alerts: the generic, pre-normalized entrypoint
// for any source that can speak AlertRequest's own JSON shape directly. It
// reads the body, unmarshals it into an AlertRequest, and delegates to
// enqueueAlert for everything else. See enqueueAlert's doc comment for the
// persist-then-respond correctness argument.
func (h *AlertHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var req AlertRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	id, alertNumber, err := h.enqueueAlert(r.Context(), req)
	h.writeEnqueueResult(w, r, id, alertNumber, err)
}

// writeEnqueueResult writes the HTTP response for an enqueueAlert result,
// shared by CreateAlert and every vendor-adapter handler in adapter_*.go so
// the 400-vs-500 classification and the 202 success body are written
// identically everywhere: a validation failure (errors.Is(err,
// errValidation)) maps to 400 with the validation message, any other error
// maps to 500 with the generic internal-error message (logged, since it's
// this service's own store failing, not a caller mistake), and success
// writes the same {"id":..., "alertNumber":...} body CreateAlert always has.
func (h *AlertHandler) writeEnqueueResult(w http.ResponseWriter, r *http.Request, id, alertNumber string, err error) {
	if err != nil {
		if errors.Is(err, errValidation) {
			writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), errValidation.Error()+": "))
			return
		}
		slog.ErrorContext(r.Context(), "handler: failed to persist buffered alert", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "alertNumber": alertNumber})
}
