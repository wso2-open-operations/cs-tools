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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"
)

func validAlertJSON() []byte {
	return []byte(`{
		"source": "azure",
		"severity": "critical",
		"service": "svc-checkout",
		"metricName": "error_rate",
		"environment": "production",
		"uniqueIdentifier": "alert-abc-123",
		"description": "Error rate exceeded 5% for 10 minutes"
	}`)
}

func TestCreateAlert_Success(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusAccepted)
	var body struct {
		ID          string `json:"id"`
		AlertNumber string `json:"alertNumber"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID == "" {
		t.Fatal("response id is empty, want a generated id")
	}
	if body.AlertNumber == "" {
		t.Fatal("response alertNumber is empty, want the generated human-readable alert number")
	}

	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
	// The id returned to the caller must be the exact same id the row was
	// persisted under.
	if store.enqueuedIDs[0] != body.ID {
		t.Errorf("enqueued id = %q, want it to match the response id %q", store.enqueuedIDs[0], body.ID)
	}
	// The alertNumber returned to the caller must be the exact same one the
	// row was persisted under — this is what lets internal/worker's later
	// SearchIncidentByTag(csmclient.DedupTag(row.AlertNumber)) find the
	// incident this same alert's Subject was tagged with below.
	if store.enqueuedAlertNumbers[0] != body.AlertNumber {
		t.Errorf("enqueued alertNumber = %q, want it to match the response alertNumber %q", store.enqueuedAlertNumbers[0], body.AlertNumber)
	}
	var incidentReq csmclient.CreateIncidentRequest
	if err := json.Unmarshal(store.enqueuedPayloads[0], &incidentReq); err != nil {
		t.Fatalf("buffered payload is not a valid CreateIncidentRequest: %v", err)
	}
	if incidentReq.CallerID != "caller-1" {
		t.Errorf("CallerID = %q, want %q", incidentReq.CallerID, "caller-1")
	}
	// No SRE_ALERT_SERVICE_MAP entry was configured for this handler (nil map
	// — see NewAlertHandler(store, "caller-1", nil) above), so the buffered
	// ServiceID must be the unresolved sentinel, never the raw "svc-checkout"
	// label — that passthrough was the bug this hybrid-resolution feature
	// exists to fix (a human-readable label is never a valid CMDB service
	// UUID). internal/worker.resolveServiceID resolves the real UUID later,
	// at delivery-attempt time, using the raw label still preserved
	// separately in the payload (see the alertpayload assertion below).
	if incidentReq.ServiceID != csmclient.UnresolvedServiceIDSentinel {
		t.Errorf("ServiceID = %q, want the unresolved sentinel %q", incidentReq.ServiceID, csmclient.UnresolvedServiceIDSentinel)
	}
	var payload struct {
		Service string `json:"service"`
	}
	if err := json.Unmarshal(store.enqueuedPayloads[0], &payload); err != nil {
		t.Fatalf("buffered payload is not valid JSON: %v", err)
	}
	if payload.Service != "svc-checkout" {
		t.Errorf("payload.Service = %q, want the raw label %q preserved for worker-side resolution", payload.Service, "svc-checkout")
	}
	if incidentReq.Impact != "HIGH" || incidentReq.Urgency != "HIGH" {
		t.Errorf("Impact/Urgency = %s/%s, want HIGH/HIGH for critical severity", incidentReq.Impact, incidentReq.Urgency)
	}
	if incidentReq.ContactType == nil || *incidentReq.ContactType != "AZURE" {
		t.Errorf("ContactType = %v, want AZURE", incidentReq.ContactType)
	}
	if incidentReq.Category != "SERVICE_INTERRUPTION" {
		t.Errorf("Category = %q, want SERVICE_INTERRUPTION (default)", incidentReq.Category)
	}
	if incidentReq.AdditionalComments == nil || *incidentReq.AdditionalComments != "Error rate exceeded 5% for 10 minutes" {
		t.Errorf("AdditionalComments = %v, want the alert description", incidentReq.AdditionalComments)
	}
	if incidentReq.WorkNotes == nil || !strings.Contains(*incidentReq.WorkNotes, "alert-abc-123") {
		t.Errorf("WorkNotes = %v, want it to contain the unique identifier", incidentReq.WorkNotes)
	}
	wantTag := csmclient.DedupTag(body.AlertNumber)
	if !strings.HasPrefix(incidentReq.Subject, wantTag) {
		t.Errorf("Subject = %q, want it to start with the dedup tag %q", incidentReq.Subject, wantTag)
	}
}

// This is the persistence-before-delivery guarantee, exercised directly:
// CreateAlert has no delivery-attempt dependency at all (no csmclient in
// sight) — a successful response is only ever contingent on Enqueue
// succeeding.
func TestCreateAlert_NeverAttemptsDeliveryInline(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusAccepted)
}

func TestCreateAlert_RejectsInvalidJSON(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader([]byte(`not json`)))
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusBadRequest)
	assertErrorMessage(t, w, ErrMsgBadRequest)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called for invalid JSON")
	}
}

func TestCreateAlert_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		// authSource is the identity the request is authenticated as -- set
		// to whatever this case's own JSON body's "source" field carries
		// (verbatim, including a missing/malformed value) so
		// requireAuthenticatedSource's own source-match check never
		// interferes with what this test is actually exercising:
		// AlertRequest.validate's 400s.
		authSource string
	}{
		{"missing source", `{"severity":"critical","service":"svc","metricName":"m","description":"d"}`, ""},
		{"missing severity", `{"source":"azure","service":"svc","metricName":"m","description":"d"}`, "azure"},
		{"missing service", `{"source":"azure","severity":"critical","metricName":"m","description":"d"}`, "azure"},
		{"missing metricName", `{"source":"azure","severity":"critical","service":"svc","description":"d"}`, "azure"},
		{"missing description", `{"source":"azure","severity":"critical","service":"svc","metricName":"m"}`, "azure"},
		{"empty body", `{}`, ""},
		// source/uniqueIdentifier are embedded verbatim in the dedup/group
		// tag (csmclient.DedupTag/GroupTag) -- a value containing the tag's
		// own delimiter characters could forge a tag colliding with a
		// different alert's group. See tagDelimiterChars's doc comment.
		{"source contains a tag delimiter", `{"source":"azure]x[group:other:uid","severity":"critical","service":"svc","metricName":"m","description":"d"}`, "azure]x[group:other:uid"},
		{"uniqueIdentifier contains a tag delimiter", `{"source":"azure","severity":"critical","service":"svc","metricName":"m","description":"d","uniqueIdentifier":"uid]x[group:other:legit-uid"}`, "azure"},
		// Source/Severity/Service/MetricName/Environment/UniqueIdentifier
		// each land in a single-line context (buildSubject, or one line of
		// buildWorkNotes) -- a newline could inject a fake extra WorkNotes
		// line (e.g. spoofing a different alert identifier). See
		// AlertRequest.validate's doc comment.
		{"source contains a newline", `{"source":"azure\nAlert identifier: forged-id","severity":"critical","service":"svc","metricName":"m","description":"d"}`, "azure\nAlert identifier: forged-id"},
		{"environment contains a newline", `{"source":"azure","severity":"critical","service":"svc","metricName":"m","description":"d","environment":"prod\nAlert identifier: forged-id"}`, "azure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockStore{}
			h := NewAlertHandler(store, "caller-1", nil)

			r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader([]byte(tc.body)))
			r = withAuthenticatedUsername(r, tc.authSource)
			w := httptest.NewRecorder()
			h.CreateAlert(w, r)

			assertStatus(t, w, http.StatusBadRequest)
			if len(store.enqueuedPayloads) != 0 {
				t.Error("Enqueue should not be called when validation fails")
			}
		})
	}
}

func TestCreateAlert_RejectsOversizedBody(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	huge := bytes.Repeat([]byte("a"), maxRequestBodyBytes+1)
	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(huge))
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusRequestEntityTooLarge)
	assertErrorMessage(t, w, ErrMsgTooLarge)
}

func TestCreateAlert_StoreFailureReturns500(t *testing.T) {
	store := &mockStore{enqueueFn: func(ctx context.Context, id string, buildPayload func(string) ([]byte, error)) (string, error) {
		return "", errors.New("connection refused")
	}}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
}

// TestCreateAlert_MismatchedSourceReturns403 is the core authorization-gap
// regression test: a caller authenticated as one source must not be able to
// submit an alert claiming to be a different one, even though the request
// body itself is otherwise perfectly well-formed.
func TestCreateAlert_MismatchedSourceReturns403(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	// validAlertJSON claims source "azure"; authenticate as a different,
	// legitimately-configured credential instead.
	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "site24x7")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusForbidden)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when the authenticated identity does not match the claimed source")
	}
}

// TestCreateAlert_SourceMatchIsCaseInsensitiveAndTrimmed confirms the
// authenticated-identity comparison isn't so strict it breaks a legitimate
// caller over case or incidental whitespace -- the credential's own
// username casing needn't exactly mirror how a vendor happens to spell its
// own Source string.
func TestCreateAlert_SourceMatchIsCaseInsensitiveAndTrimmed(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "  AZURE  ")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Errorf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}
}

// TestCreateAlert_NoAuthenticatedUsernameReturns500 covers the defensive
// path: this should never happen in production, since main.go wraps every
// route this handler serves in the BasicAuth middleware, but a missing
// authenticated identity must fail closed (500, logged) rather than silently
// let the request through unauthorized.
func TestCreateAlert_NoAuthenticatedUsernameReturns500(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusInternalServerError)
	assertErrorMessage(t, w, ErrMsgInternal)
	if len(store.enqueuedPayloads) != 0 {
		t.Error("Enqueue should not be called when there is no authenticated identity in context")
	}
}

func TestMapToIncident_UnmappedSourceOmitsContactType(t *testing.T) {
	req := AlertRequest{Source: "datadog", Severity: "minor", Service: "svc", MetricName: "m", Description: "d"}
	out := MapToIncident(req, "alert-id-1", "caller-1", nil)
	if out.ContactType != nil {
		t.Errorf("ContactType = %v, want nil for an unmapped source", out.ContactType)
	}
}

func TestMapToIncident_CategoryPassthroughWhenValid(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "minor", Service: "svc", MetricName: "m", Description: "d", Category: "security"}
	out := MapToIncident(req, "alert-id-1", "caller-1", nil)
	if out.Category != "SECURITY" {
		t.Errorf("Category = %q, want SECURITY", out.Category)
	}
}

// TestMapToIncident_ServiceMapHitResolvesServiceID pins the static-map fast
// path of the hybrid service-UUID resolution design: an exact match in
// serviceMap is used immediately, synchronously — no sentinel, no deferred
// resolution needed.
func TestMapToIncident_ServiceMapHitResolvesServiceID(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "minor", Service: "Azure Monitoring", MetricName: "m", Description: "d"}
	serviceMap := map[string]string{"Azure Monitoring": "33333333-3333-3333-3333-333333333333"}
	out := MapToIncident(req, "alert-id-1", "caller-1", serviceMap)
	if out.ServiceID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("ServiceID = %q, want the static-map match", out.ServiceID)
	}
}

// TestMapToIncident_ServiceMapMissUsesSentinel pins the other half: a label
// with no static-map entry (including a nil map entirely) gets
// csmclient.UnresolvedServiceIDSentinel, never the raw label itself — the
// bug this whole feature exists to fix was exactly "the raw label was sent
// as ServiceID verbatim." internal/worker's live resolution is what turns
// this into a real UUID later, not this function.
func TestMapToIncident_ServiceMapMissUsesSentinel(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "minor", Service: "Some Unmapped Service", MetricName: "m", Description: "d"}

	for name, serviceMap := range map[string]map[string]string{
		"nil map":            nil,
		"non-matching entry": {"A Different Service": "33333333-3333-3333-3333-333333333333"},
	} {
		t.Run(name, func(t *testing.T) {
			out := MapToIncident(req, "alert-id-1", "caller-1", serviceMap)
			if out.ServiceID != csmclient.UnresolvedServiceIDSentinel {
				t.Errorf("ServiceID = %q, want the unresolved sentinel %q", out.ServiceID, csmclient.UnresolvedServiceIDSentinel)
			}
			if out.ServiceID == req.Service {
				t.Errorf("ServiceID must never be the raw, non-UUID Service label — got %q", out.ServiceID)
			}
		})
	}
}

// TestMapToIncident_ImpactUrgencyOverride pins the additive
// AlertRequest.Impact/Urgency override: when set, they replace
// severity.MapImpactUrgency's own derivation for that field; when nil (every
// existing caller today), MapToIncident's output is unchanged from before
// this override existed.
func TestMapToIncident_ImpactUrgencyOverride(t *testing.T) {
	t.Run("nil Impact/Urgency falls back to severity-derived values unchanged", func(t *testing.T) {
		req := AlertRequest{Source: "azure", Severity: "minor", Service: "svc", MetricName: "m", Description: "d"}
		out := MapToIncident(req, "alert-id-1", "caller-1", nil)
		if out.Impact != "MEDIUM" || out.Urgency != "MEDIUM" {
			t.Errorf("Impact/Urgency = %q/%q, want MEDIUM/MEDIUM (severity.MapImpactUrgency(\"minor\"), unchanged)", out.Impact, out.Urgency)
		}
	})

	t.Run("explicit Impact/Urgency override the severity-derived values", func(t *testing.T) {
		impact, urgency := "HIGH", "LOW"
		req := AlertRequest{Source: "azure", Severity: "minor", Service: "svc", MetricName: "m", Description: "d", Impact: &impact, Urgency: &urgency}
		out := MapToIncident(req, "alert-id-1", "caller-1", nil)
		if out.Impact != "HIGH" {
			t.Errorf("Impact = %q, want the explicit override HIGH (not minor's usual MEDIUM)", out.Impact)
		}
		if out.Urgency != "LOW" {
			t.Errorf("Urgency = %q, want the explicit override LOW (not minor's usual MEDIUM)", out.Urgency)
		}
	})

	t.Run("only Impact set overrides just that field", func(t *testing.T) {
		impact := "HIGH"
		req := AlertRequest{Source: "azure", Severity: "minor", Service: "svc", MetricName: "m", Description: "d", Impact: &impact}
		out := MapToIncident(req, "alert-id-1", "caller-1", nil)
		if out.Impact != "HIGH" {
			t.Errorf("Impact = %q, want the explicit override HIGH", out.Impact)
		}
		if out.Urgency != "MEDIUM" {
			t.Errorf("Urgency = %q, want the severity-derived MEDIUM (Urgency was never overridden)", out.Urgency)
		}
	})
}

// TestAlertRequest_validate_RejectsInvalidImpactUrgency pins the fix for a
// real gap: validate() previously never checked Impact/Urgency, so a
// generic /alerts caller could submit any string in those fields and have
// it forwarded straight into CSM's own incident contract, which only
// accepts HIGH/MEDIUM/LOW.
func TestAlertRequest_validate_RejectsInvalidImpactUrgency(t *testing.T) {
	base := func() AlertRequest {
		return AlertRequest{Source: "azure", Severity: "minor", Service: "svc", MetricName: "m", Description: "d"}
	}

	t.Run("nil Impact/Urgency is valid", func(t *testing.T) {
		if msg := base().validate(); msg != "" {
			t.Errorf("validate() = %q, want \"\"", msg)
		}
	})

	t.Run("valid Impact/Urgency values are accepted", func(t *testing.T) {
		for _, v := range []string{"HIGH", "MEDIUM", "LOW"} {
			req := base()
			req.Impact = &v
			req.Urgency = &v
			if msg := req.validate(); msg != "" {
				t.Errorf("validate() with Impact=Urgency=%q = %q, want \"\"", v, msg)
			}
		}
	})

	t.Run("invalid Impact is rejected", func(t *testing.T) {
		bad := "CRITICAL"
		req := base()
		req.Impact = &bad
		if msg := req.validate(); msg != "impact must be HIGH, MEDIUM, or LOW" {
			t.Errorf("validate() = %q, want the impact error", msg)
		}
	})

	t.Run("invalid Urgency is rejected", func(t *testing.T) {
		bad := ""
		req := base()
		req.Urgency = &bad
		if msg := req.validate(); msg != "urgency must be HIGH, MEDIUM, or LOW" {
			t.Errorf("validate() = %q, want the urgency error", msg)
		}
	})
}

// The dedup tag is the load-bearing contract internal/worker's
// SearchIncidentByTag pre-retry check depends on (see
// internal/csmclient.DedupTag's doc comment) — this pins the exact format
// so a future Subject-formatting tweak can't silently break it.
func TestMapToIncident_SubjectStartsWithDedupTag(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "critical", Service: "svc-checkout", MetricName: "error_rate", Description: "d"}
	out := MapToIncident(req, "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", "caller-1", nil)
	want := "[alert:1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed]"
	if !strings.HasPrefix(out.Subject, want) {
		t.Errorf("Subject = %q, want it to start with %q", out.Subject, want)
	}
	if out.Subject != csmclient.DedupTag("1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed")+" [azure] error_rate alert: svc-checkout" {
		t.Errorf("Subject = %q, unexpected full format", out.Subject)
	}
}

// TestMapToIncident_SubjectIncludesGroupTagWhenUniqueIdentifierSet pins the
// contract internal/worker.tryGroup's search depends on
// (csmclient.GroupTag's doc comment): an alert with a UniqueIdentifier gets
// both tags in its Subject, dedup tag first, group tag second.
func TestMapToIncident_SubjectIncludesGroupTagWhenUniqueIdentifierSet(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "critical", Service: "svc-checkout", MetricName: "error_rate", Description: "d", UniqueIdentifier: "uid-123"}
	out := MapToIncident(req, "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", "caller-1", nil)
	want := csmclient.DedupTag("1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed") + " " + csmclient.GroupTag("azure", "uid-123") + " [azure] error_rate alert: svc-checkout"
	if out.Subject != want {
		t.Errorf("Subject = %q, want %q", out.Subject, want)
	}
}

// TestMapToIncident_SubjectOmitsGroupTagWhenNoUniqueIdentifier confirms the
// group tag is left out entirely (not an empty "[group::]") when there's
// nothing to group by — internal/worker.attempt already gates tryGroup on
// UniqueIdentifier != "", but the Subject itself must not carry a
// meaningless tag either.
func TestMapToIncident_SubjectOmitsGroupTagWhenNoUniqueIdentifier(t *testing.T) {
	req := AlertRequest{Source: "azure", Severity: "critical", Service: "svc-checkout", MetricName: "error_rate", Description: "d"}
	out := MapToIncident(req, "1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed", "caller-1", nil)
	if strings.Contains(out.Subject, "[group:") {
		t.Errorf("Subject = %q, want no group tag when UniqueIdentifier is empty", out.Subject)
	}
}

// TestDeriveAlertStatus pins deriveAlertStatus's severity->FIRING/RESOLVED
// mapping: "ok" (case-insensitively, surrounding whitespace trimmed) is the
// sole severity value treated as RESOLVED, matching the one signal
// internal/severity.MapImpactUrgency already documents as meaning "this
// condition has cleared." Every other severity value, including one this
// service doesn't otherwise recognize, means the condition is still active.
func TestDeriveAlertStatus(t *testing.T) {
	cases := []struct {
		severity string
		want     string
	}{
		{"ok", "RESOLVED"},
		{"OK", "RESOLVED"},
		{"  ok  ", "RESOLVED"},
		{"critical", "FIRING"},
		{"major", "FIRING"},
		{"minor", "FIRING"},
		{"warning", "FIRING"},
		{"", "FIRING"},
		{"some-unrecognized-value", "FIRING"},
	}
	for _, tc := range cases {
		t.Run(tc.severity, func(t *testing.T) {
			got := deriveAlertStatus(AlertRequest{Severity: tc.severity})
			if got != tc.want {
				t.Errorf("deriveAlertStatus(Severity=%q) = %q, want %q", tc.severity, got, tc.want)
			}
		})
	}
}

// TestCreateAlert_PersistsGroupingFieldsAlongsideMappedIncident verifies the
// buffered payload carries the alertpayload.Payload superset fields
// (source/uniqueIdentifier/service/metricName/alertStatus), not just the
// embedded CreateIncidentRequest — internal/worker's incident-grouping logic
// depends on these being present in what actually gets persisted.
func TestCreateAlert_PersistsGroupingFieldsAlongsideMappedIncident(t *testing.T) {
	store := &mockStore{}
	h := NewAlertHandler(store, "caller-1", nil)

	r := httptest.NewRequest(http.MethodPost, "/alerts", bytes.NewReader(validAlertJSON()))
	r = withAuthenticatedUsername(r, "azure")
	w := httptest.NewRecorder()
	h.CreateAlert(w, r)

	assertStatus(t, w, http.StatusAccepted)
	if len(store.enqueuedPayloads) != 1 {
		t.Fatalf("Enqueue called %d times, want 1", len(store.enqueuedPayloads))
	}

	var got struct {
		Source           string `json:"source"`
		UniqueIdentifier string `json:"uniqueIdentifier"`
		Service          string `json:"service"`
		MetricName       string `json:"metricName"`
		AlertStatus      string `json:"alertStatus"`
	}
	if err := json.Unmarshal(store.enqueuedPayloads[0], &got); err != nil {
		t.Fatalf("buffered payload is not valid JSON: %v", err)
	}
	if got.Source != "azure" || got.UniqueIdentifier != "alert-abc-123" || got.Service != "svc-checkout" ||
		got.MetricName != "error_rate" || got.AlertStatus != "FIRING" {
		t.Errorf("persisted grouping fields = %+v, want source=azure uniqueIdentifier=alert-abc-123 service=svc-checkout metricName=error_rate alertStatus=FIRING", got)
	}
}
