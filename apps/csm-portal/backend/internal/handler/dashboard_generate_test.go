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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// withGenerateSample swaps in a registry that also holds the stub's sample
// dashboard, restored via t.Cleanup -- the same pattern as
// withRestrictedDashboard, so the shared two-dashboard fixture other tests
// count on is never changed permanently.
func withGenerateSample(t *testing.T) {
	t.Helper()
	previous := dashboard.Active()
	t.Cleanup(func() { dashboard.SetActive(previous) })

	sample := dashboard.Dashboard{
		ID:          generateSampleDashboardID,
		DisplayName: "My Cases and Escalations",
		Type:        dashboard.TypeCS,
		Widgets: []dashboard.WidgetTemplate{
			{
				ID:           "my-open-cases-by-severity",
				DisplayName:  "My Open Cases by Severity",
				ResourceType: dashboard.ResourceCase,
				Shape:        dashboard.ShapeCount,
				GridWidth:    6,
				Query:        map[string]any{"filters": []any{}},
			},
			{
				ID:           "recent-escalations",
				DisplayName:  "Recent Escalations",
				ResourceType: dashboard.ResourceCase,
				Shape:        dashboard.ShapeList,
				GridWidth:    6,
				ListLimit:    10,
				Query:        map[string]any{"filters": []any{}},
			},
		},
	}
	dashboards := append(append([]dashboard.Dashboard(nil), dashboard.All()...), sample)
	dashboard.SetActive(dashboard.NewStaticRegistry(dashboards))
}

func newGenerateRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/dashboards/generate", strings.NewReader(body))
}

func TestGenerateDashboard_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(w, newGenerateRequest(`{"prompt":"x"}`))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestGenerateDashboard_InvalidJSONIsBadRequest(t *testing.T) {
	withGenerateSample(t)
	w := httptest.NewRecorder()
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGenerateDashboard_SampleMissingIsServiceUnavailable(t *testing.T) {
	// The shared fixture has no sample dashboard.
	w := httptest.NewRecorder()
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":"x"}`)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	body := decodeJSON[map[string]any](t, w)
	if body["message"] != errMsgGenerateUnavailable {
		t.Fatalf("message = %v, want %q", body["message"], errMsgGenerateUnavailable)
	}
}

func TestGenerateDashboard_ReturnsTheSampleAndAReply(t *testing.T) {
	withGenerateSample(t)
	w := httptest.NewRecorder()
	req := withUser(newGenerateRequest(`{"prompt":"My open cases by severity","current":{"id":"x","widgets":[]}}`))
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	resp := decodeJSON[generateDashboardResponse](t, w)
	if !uuidRe.MatchString(resp.Dashboard.ID) || resp.Dashboard.ID == generateSampleDashboardID {
		t.Fatalf("dashboard.id = %q, want a new UUID (the draft's id %q is not one)", resp.Dashboard.ID, "x")
	}
	if len(resp.Dashboard.Widgets) != 2 {
		t.Fatalf("widgets = %d, want 2", len(resp.Dashboard.Widgets))
	}
	if !strings.Contains(resp.Reply, "My Cases and Escalations, 2 widgets") {
		t.Fatalf("reply = %q, want it to name the dashboard and its widget count", resp.Reply)
	}
}

// The preview must render exactly what the dashboard looks like everywhere
// else, so the stub's dashboard is the same JSON GET /dashboards/{id} returns
// (apart from its id).
func TestGenerateDashboard_DashboardMatchesGetDashboardDetail(t *testing.T) {
	withGenerateSample(t)

	gen := httptest.NewRecorder()
	NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(gen, withUser(newGenerateRequest(`{"prompt":"x"}`)))
	var genBody struct {
		Dashboard json.RawMessage `json:"dashboard"`
	}
	if err := json.Unmarshal(gen.Body.Bytes(), &genBody); err != nil {
		t.Fatalf("decode generate response: %v", err)
	}

	detailReq := withUser(httptest.NewRequest(http.MethodGet, "/dashboards/"+generateSampleDashboardID, nil))
	detailReq.SetPathValue("dashboardId", generateSampleDashboardID)
	detail := httptest.NewRecorder()
	NewDashboardHandler(nil).GetDashboardDetail(detail, detailReq)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", detail.Code)
	}

	// Everything but the id, which the generate endpoint assigns itself.
	var a, b map[string]any
	if err := json.Unmarshal(genBody.Dashboard, &a); err != nil {
		t.Fatalf("decode generated dashboard: %v", err)
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	delete(a, "id")
	delete(b, "id")
	ga, _ := json.Marshal(a)
	gb, _ := json.Marshal(b)
	if string(ga) != string(gb) {
		t.Fatalf("generated dashboard differs from GET /dashboards/{id}:\n got: %s\nwant: %s", ga, gb)
	}
}

func TestGenerateDashboard_PromptChecks(t *testing.T) {
	withGenerateSample(t)

	// One character, three bytes in UTF-8, so byte length and character length differ.
	sinhala := "ඩ"
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantMsg    string
	}{
		{"missing prompt", `{}`, http.StatusBadRequest, errMsgGeneratePromptEmpty},
		{"empty prompt", `{"prompt":""}`, http.StatusBadRequest, errMsgGeneratePromptEmpty},
		{"spaces only", `{"prompt":"   \n\t "}`, http.StatusBadRequest, errMsgGeneratePromptEmpty},
		{"exactly the limit", promptBody(strings.Repeat("a", generatePromptMaxLength)), http.StatusOK, ""},
		{"one over the limit", promptBody(strings.Repeat("a", generatePromptMaxLength+1)), http.StatusBadRequest, errMsgGeneratePromptTooLong},
		{"surrounding spaces don't count", promptBody("  " + strings.Repeat("a", generatePromptMaxLength) + "  "), http.StatusOK, ""},
		// 2,000 characters but well over 2,000 bytes: counted in characters.
		{"non-Latin at the limit", promptBody(strings.Repeat(sinhala, generatePromptMaxLength)), http.StatusOK, ""},
		{"non-Latin one over", promptBody(strings.Repeat(sinhala, generatePromptMaxLength+1)), http.StatusBadRequest, errMsgGeneratePromptTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewDashboardGenerateHandler(NewSampleDashboardGenerator()).GenerateDashboard(w, withUser(newGenerateRequest(tc.body)))
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantMsg != "" {
				body := decodeJSON[map[string]any](t, w)
				if body["message"] != tc.wantMsg {
					t.Fatalf("message = %v, want %q", body["message"], tc.wantMsg)
				}
			}
		})
	}
}

// promptBody returns {"prompt": p} as JSON.
func promptBody(p string) string {
	b, _ := json.Marshal(map[string]string{"prompt": p})
	return string(b)
}

// generateTestDraftID is a draft's UUID id, upper case to check it comes back
// normalised.
const generateTestDraftID = "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE"

// fakeGenerator records what the handler passed and answers with result/err.
type fakeGenerator struct {
	gotPrompt  string
	gotCurrent json.RawMessage
	calls      int
	result     GeneratedDashboard
	err        error
}

func (f *fakeGenerator) Generate(_ context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	f.calls++
	f.gotPrompt, f.gotCurrent = prompt, current
	return f.result, f.err
}

func TestGenerateDashboard_PassesTheTrimmedPromptAndDraftToTheGenerator(t *testing.T) {
	fake := &fakeGenerator{result: GeneratedDashboard{
		Dashboard: dashboard.Dashboard{ID: "model-chosen-id", DisplayName: "X", Widgets: []dashboard.WidgetTemplate{{
			ID: "w1", DisplayName: "W1", ResourceType: dashboard.ResourceCase, Shape: dashboard.ShapeCount, GridWidth: 4,
		}}},
		Reply: "Here you go.",
	}}
	w := httptest.NewRecorder()
	NewDashboardGenerateHandler(fake).GenerateDashboard(w,
		withUser(newGenerateRequest(`{"prompt":"  my cases  ","current":{"id":"`+generateTestDraftID+`"}}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if fake.gotPrompt != "my cases" {
		t.Errorf("prompt = %q, want the trimmed %q", fake.gotPrompt, "my cases")
	}
	if string(fake.gotCurrent) != `{"id":"`+generateTestDraftID+`"}` {
		t.Errorf("current = %s, want the draft as sent", fake.gotCurrent)
	}
	resp := decodeJSON[generateDashboardResponse](t, w)
	if resp.Reply != "Here you go." || resp.Dashboard.ID != strings.ToLower(generateTestDraftID) || len(resp.Dashboard.Widgets) != 1 ||
		resp.Dashboard.Widgets[0].WidgetID != "w1" {
		t.Errorf("response = %+v, want the generator's dashboard (wire format) and reply", resp)
	}
}

func TestGenerateDashboard_RefusedPromptNeverReachesTheGenerator(t *testing.T) {
	fake := &fakeGenerator{}
	w := httptest.NewRecorder()
	NewDashboardGenerateHandler(fake).GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":"  "}`)))
	if w.Code != http.StatusBadRequest || fake.calls != 0 {
		t.Fatalf("status = %d, generator calls = %d; want 400 and 0", w.Code, fake.calls)
	}
}

func TestGenerateDashboard_GeneratorErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"unavailable", fmt.Errorf("wrapped: %w", ErrGenerateUnavailable), http.StatusServiceUnavailable, errMsgGenerateUnavailable},
		{"any other failure", errors.New("boom"), http.StatusInternalServerError, ErrMsgInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewDashboardGenerateHandler(&fakeGenerator{err: tc.err}).GenerateDashboard(w,
				withUser(newGenerateRequest(`{"prompt":"x"}`)))
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			body := decodeJSON[map[string]any](t, w)
			if body["message"] != tc.wantMsg {
				t.Fatalf("message = %v, want %q (no internal detail)", body["message"], tc.wantMsg)
			}
		})
	}
}

func TestGeneratedDashboardID(t *testing.T) {
	kept := []struct{ current, want string }{
		{`{"id":"` + generateTestDraftID + `","widgets":[]}`, strings.ToLower(generateTestDraftID)},
		{`{"id":"11111111-2222-4333-8444-555555555555"}`, "11111111-2222-4333-8444-555555555555"},
	}
	for _, tc := range kept {
		got, err := generatedDashboardID(json.RawMessage(tc.current))
		if err != nil || got != tc.want {
			t.Errorf("generatedDashboardID(%s) = %q, %v; want the draft's id %q", tc.current, got, err, tc.want)
		}
	}

	seen := map[string]bool{}
	for _, current := range []string{"", "null", `{"id":"ai-my-cases"}`, `{"id":""}`, `{"widgets":[]}`, `{"id":`, `["x"]`} {
		got, err := generatedDashboardID(json.RawMessage(current))
		if err != nil {
			t.Fatalf("generatedDashboardID(%q): %v", current, err)
		}
		if !uuidRe.MatchString(got) || got[14] != '4' || !strings.ContainsRune("89ab", rune(got[19])) {
			t.Errorf("generatedDashboardID(%q) = %q, want a new version 4 UUID", current, got)
		}
		if seen[got] {
			t.Errorf("generatedDashboardID(%q) = %q, repeated", current, got)
		}
		seen[got] = true
	}
}

func TestGenerateDashboard_MapsEachGeneratorError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"invalid draft", fmt.Errorf("%w: bad", errDraftInvalid), http.StatusBadRequest, errMsgGenerateDraftInvalid},
		{"draft too large", fmt.Errorf("%w: 70000 bytes", errDraftTooLarge), http.StatusRequestEntityTooLarge, errMsgGenerateDraftTooBig},
		{"declined", &generateDeclinedError{Reason: "Resolution time per engineer is not in the catalogue."}, http.StatusUnprocessableEntity, "Resolution time per engineer is not in the catalogue."},
		{"refused", errGenerateRefused, http.StatusUnprocessableEntity, errMsgGenerateRefused},
		{"team filter", fmt.Errorf("%w: widgets[0]: unknown team %q (known teams: rigel, sirius)", errDashboardTeam, "orion"), http.StatusUnprocessableEntity, `Unknown team "orion" (known teams: rigel, sirius).`},
		{"unusable", fmt.Errorf("%w: %w", errGenerateUnusable, dashboard.ErrGeneratedInvalid), http.StatusBadGateway, errMsgGenerateUnusable},
		{"truncated", errGenerateTruncated, http.StatusBadGateway, errMsgGenerateUnusable},
		{"call timed out", fmt.Errorf("%w: %w", errGenerateCall, context.DeadlineExceeded), http.StatusGatewayTimeout, errMsgGenerateTimeout},
		{"call failed", fmt.Errorf("%w: 401 Unauthorized", errGenerateCall), http.StatusBadGateway, errMsgGenerateCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewDashboardGenerateHandler(&fakeGenerator{err: tc.err}).GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":"x"}`)))
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if body := decodeJSON[map[string]any](t, w); body["message"] != tc.wantMsg {
				t.Fatalf("message = %v, want %q", body["message"], tc.wantMsg)
			}
		})
	}
}

// End to end through the real csmai client and a fake AI integration
// service: what the service answers for each outcome reaches the right status
// and message.
func TestGenerateDashboard_ThroughTheAIIntegrationService(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		timeout    time.Duration
		delay      time.Duration
		wantStatus int
		wantMsg    string
	}{
		{"dashboard", http.StatusOK, `{"candidates":[` + validGenerated + `],"usage":{}}`, 5 * time.Second, 0, http.StatusOK, ""},
		{"declined", http.StatusOK, `{"declined":"Not in the catalogue.","usage":{}}`, 5 * time.Second, 0, http.StatusUnprocessableEntity, "Not in the catalogue."},
		{"no valid candidate", http.StatusOK, `{"candidates":[{"displayName":"x"}],"usage":{}}`, 5 * time.Second, 0, http.StatusBadGateway, errMsgGenerateUnusable},
		{"refused", http.StatusUnprocessableEntity, `{"code":"refused","message":"m"}`, 5 * time.Second, 0, http.StatusUnprocessableEntity, errMsgGenerateRefused},
		{"model timed out", http.StatusGatewayTimeout, `{"code":"timeout","message":"m"}`, 5 * time.Second, 0, http.StatusGatewayTimeout, errMsgGenerateTimeout},
		{"no key in the service", http.StatusServiceUnavailable, `{"code":"not_configured","message":"m"}`, 5 * time.Second, 0, http.StatusServiceUnavailable, errMsgGenerateUnavailable},
		{"service unauthorised", http.StatusUnauthorized, `{"error":"Unauthorized"}`, 5 * time.Second, 0, http.StatusBadGateway, errMsgGenerateCall},
		{"service too slow", http.StatusOK, `{"candidates":[` + validGenerated + `]}`, 200 * time.Millisecond, 3 * time.Second, http.StatusGatewayTimeout, errMsgGenerateTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"access_token":"t","token_type":"Bearer","expires_in":3600}`)
					return
				}
				if tc.delay > 0 {
					select {
					case <-time.After(tc.delay):
					case <-r.Context().Done():
						return
					}
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			client := csmai.NewClient(csmai.Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "s", Timeout: tc.timeout})
			gen := NewServiceDashboardGenerator(client, nil)
			w := httptest.NewRecorder()
			NewDashboardGenerateHandler(gen).GenerateDashboard(w, withUser(newGenerateRequest(`{"prompt":"Top 10 customers by open cases."}`)))
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				if !strings.Contains(w.Body.String(), tc.wantMsg) {
					t.Errorf("body = %s, want the message %q", w.Body.String(), tc.wantMsg)
				}
				return
			}
			resp := decodeJSON[generateDashboardResponse](t, w)
			if !uuidRe.MatchString(resp.Dashboard.ID) || resp.Dashboard.DisplayName != "Top Customers by Open Cases" ||
				len(resp.Dashboard.Widgets) != 1 || resp.Dashboard.Widgets[0].WidgetID != "top" {
				t.Errorf("response = %+v, want the generated dashboard in the served form with a UUID id", resp.Dashboard)
			}
			if !strings.Contains(fmt.Sprint(resp.Dashboard.Widgets[0].Query), "case") {
				t.Errorf("query = %v, want the implied type filter injected", resp.Dashboard.Widgets[0].Query)
			}
		})
	}
}
