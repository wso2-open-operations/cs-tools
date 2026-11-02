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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

type fakeReminderSettingsClient struct {
	family string
	body   map[string]any
	reply  []byte
	err    error
}

func (f *fakeReminderSettingsClient) GetRotaReminderSettings(_ context.Context, family string) ([]byte, error) {
	f.family = family
	return f.reply, f.err
}

func (f *fakeReminderSettingsClient) PutRotaReminderSettings(_ context.Context, family string, body []byte) ([]byte, error) {
	f.family = family
	_ = json.Unmarshal(body, &f.body)
	return f.reply, f.err
}

func reminderSettingsRequest(method, family, body string) *http.Request {
	r := httptest.NewRequest(method, "/team-schedule/reminder-settings/"+family, strings.NewReader(body))
	r.SetPathValue("family", family)
	return withUser(r)
}

const goodReminderBody = `{"weekdayEnabled":true,"weekendEnabled":true,"invitesEnabled":false,
	"weekendRequired":{"CRE_WEEKEND":3},"windows":{"CRE_WEEKEND_NIGHT":{"start":"08:00","end":"18:00","timeZone":"America/Los_Angeles"}},
	"copyTo":["lead@example.com"],"sneaky":"dropped"}`

// Only the form's fields go on, to the upper-cased family.
func TestRotaReminderSettings_ForwardsTheKnownFields(t *testing.T) {
	fake := &fakeReminderSettingsClient{reply: []byte(`{"family":"CRE"}`)}
	w := httptest.NewRecorder()
	NewRotaReminderSettingsHandler(fake).PutRotaReminderSettings(w, reminderSettingsRequest(http.MethodPut, "cre", goodReminderBody))
	if w.Code != http.StatusOK || fake.family != "CRE" {
		t.Fatalf("status %d family %q: %s", w.Code, fake.family, w.Body)
	}
	if _, ok := fake.body["sneaky"]; ok || fake.body["invitesEnabled"] != false || fake.body["copyTo"].([]any)[0] != "lead@example.com" {
		t.Errorf("forwarded %v", fake.body)
	}
}

func TestRotaReminderSettings_RefusesABadRequestWithoutCallingOut(t *testing.T) {
	for name, c := range map[string]struct{ family, body string }{
		"unknown family": {"OPS", goodReminderBody},
		"missing switch": {"CRE", `{"weekdayEnabled":true,"weekendEnabled":true}`},
		"wrong type":     {"CRE", `{"weekdayEnabled":"yes","weekendEnabled":true,"invitesEnabled":true}`},
	} {
		fake := &fakeReminderSettingsClient{}
		w := httptest.NewRecorder()
		NewRotaReminderSettingsHandler(fake).PutRotaReminderSettings(w, reminderSettingsRequest(http.MethodPut, c.family, c.body))
		if w.Code != http.StatusBadRequest || fake.family != "" {
			t.Errorf("%s: status %d, called %q", name, w.Code, fake.family)
		}
	}
}

// Nothing saved stays a 404 (the card shows the built-in settings); a refusal
// keeps entity-service's reason; anything else is the fixed message.
func TestRotaReminderSettings_PassesOnWhatTheCardNeeds(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		says   string
	}{
		{"nothing saved", &apierror.Error{StatusCode: 404, Body: `{"code":404,"message":"no rota reminder settings saved for this family"}`}, 404, "no rota reminder settings saved"},
		{"refused", &apierror.Error{StatusCode: 403, Body: `{"code":403,"message":"only a CSM admin or the CRE rota admin can change the CRE rota reminder settings"}`}, 403, "only a CSM admin or the CRE rota admin"},
		{"broken", &apierror.Error{StatusCode: 500, Body: `{"code":500,"message":"pq: secret detail"}`}, 500, "Failed to"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		NewRotaReminderSettingsHandler(&fakeReminderSettingsClient{err: c.err}).GetRotaReminderSettings(w, reminderSettingsRequest(http.MethodGet, "CRE", ""))
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.says) || strings.Contains(w.Body.String(), "secret detail") {
			t.Errorf("%s: %d %s", c.name, w.Code, w.Body)
		}
	}
}

func TestRotaReminderSettings_NeedsAUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/team-schedule/reminder-settings/CRE", nil)
	r.SetPathValue("family", "CRE")
	w := httptest.NewRecorder()
	NewRotaReminderSettingsHandler(&fakeReminderSettingsClient{}).GetRotaReminderSettings(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d", w.Code)
	}
}

type fakeReminderRequestClient struct {
	family string
	body   map[string]any
	err    error
}

func (f *fakeReminderRequestClient) ListRotaReminderRequests(_ context.Context, family string) ([]byte, error) {
	f.family = family
	return []byte(`{"requests":[]}`), f.err
}

func (f *fakeReminderRequestClient) CreateRotaReminderRequest(_ context.Context, family string, body []byte) ([]byte, error) {
	f.family = family
	_ = json.Unmarshal(body, &f.body)
	return []byte(`{"status":"QUEUED"}`), f.err
}

// Only weekday or weekend goes on, to the upper-cased family; a bad request
// never calls out; a refusal keeps entity-service's reason, a 429 included.
func TestRotaReminderRequest_SendNow(t *testing.T) {
	fake := &fakeReminderRequestClient{}
	w := httptest.NewRecorder()
	NewRotaReminderRequestHandler(fake).CreateRotaReminderRequest(w, reminderSettingsRequest(http.MethodPost, "cre", `{"kind":"weekend","extra":1}`))
	if w.Code != http.StatusCreated || fake.family != "CRE" || len(fake.body) != 2 || fake.body["kind"] != "weekend" || fake.body["test"] != false {
		t.Fatalf("status %d family %q body %v", w.Code, fake.family, fake.body)
	}
	fake = &fakeReminderRequestClient{}
	w = httptest.NewRecorder()
	NewRotaReminderRequestHandler(fake).CreateRotaReminderRequest(w, reminderSettingsRequest(http.MethodPost, "CRE", `{"kind":"weekday","test":true}`))
	if w.Code != http.StatusCreated || fake.body["test"] != true {
		t.Fatalf("a test: status %d body %v", w.Code, fake.body)
	}
	for _, kind := range []string{"morning", "evening"} {
		fake = &fakeReminderRequestClient{}
		w = httptest.NewRecorder()
		NewRotaReminderRequestHandler(fake).CreateRotaReminderRequest(w, reminderSettingsRequest(http.MethodPost, "CRE", `{"kind":"`+kind+`"}`))
		if w.Code != http.StatusCreated || fake.body["kind"] != kind {
			t.Fatalf("%s: status %d body %v", kind, w.Code, fake.body)
		}
	}
	for _, bad := range []struct{ family, body string }{{"OPS", `{"kind":"weekend"}`}, {"CRE", `{"kind":"monthly"}`}, {"CRE", `{"kind":"Morning"}`}, {"CRE", `{}`}, {"CRE", `{"kind":"weekend","test":"yes"}`}} {
		fake := &fakeReminderRequestClient{}
		w := httptest.NewRecorder()
		NewRotaReminderRequestHandler(fake).CreateRotaReminderRequest(w, reminderSettingsRequest(http.MethodPost, bad.family, bad.body))
		if w.Code != http.StatusBadRequest || fake.family != "" {
			t.Errorf("%+v: status %d, called %q", bad, w.Code, fake.family)
		}
	}
	for _, status := range []int{409, 429, 403} {
		fake := &fakeReminderRequestClient{err: &apierror.Error{StatusCode: status, Body: `{"code":1,"message":"a weekend reminder was asked for less than 10 minutes ago; try again later"}`}}
		w := httptest.NewRecorder()
		NewRotaReminderRequestHandler(fake).CreateRotaReminderRequest(w, reminderSettingsRequest(http.MethodPost, "CRE", `{"kind":"weekend"}`))
		if w.Code != status || !strings.Contains(w.Body.String(), "less than 10 minutes ago") {
			t.Errorf("%d: %d %s", status, w.Code, w.Body)
		}
	}
	w = httptest.NewRecorder()
	NewRotaReminderRequestHandler(&fakeReminderRequestClient{}).ListRotaReminderRequests(w, reminderSettingsRequest(http.MethodGet, "CRE", ""))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "requests") {
		t.Errorf("list: %d %s", w.Code, w.Body)
	}
}
