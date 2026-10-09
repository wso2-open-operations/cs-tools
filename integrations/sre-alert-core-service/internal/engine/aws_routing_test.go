// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
	"alert-core-service/internal/notify"
)

// Stored alerts exactly as sre-alert-ingestion-service writes them for one CloudWatch alarm
// (prod-rds-cpu-utilization-high, account 487629103847, SNS topic sre-apollo-alarms), posted to
// /aws; captured from that service's own transform and ingest code.
const awsAlertBase = `"metric_name":"prod-rds-cpu-utilization-high","severity":"critical","category":"service_interruption","environment":"Unknown","source":"AWS","unique_identifier":"arn:aws:cloudwatch:us-east-1:487629103847:alarm:prod-rds-cpu-utilization-high","description":"(alarm message)","source_topic":"arn:aws:sns:us-east-1:487629103847:sre-apollo-alarms","source_account":"487629103847"`

var awsStoredAlerts = map[string]string{
	"named":     `{"service":"choreo-control-plane",` + awsAlertBase + `,"assignment_group":"SRE - Artemis"}`,
	"service":   `{"service":"choreo-control-plane",` + awsAlertBase + `}`,
	"unmatched": `{"service":"no-such-service",` + awsAlertBase + `}`,
	"account":   `{"service":"",` + awsAlertBase + `}`,
}

// fakeCSM is csm-integration-service as alert-core sees it: an OAuth2 token endpoint, the CMDB
// service search, the incident search, and the incident create, whose request bodies it keeps.
type fakeCSM struct {
	mu      sync.Mutex
	creates []map[string]any
	// failServiceSearch makes /services/search answer 500, as a CSM outage would.
	failServiceSearch bool
	// incidentSearches counts /incidents/search calls.
	incidentSearches int
}

func (f *fakeCSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/token":
		_, _ = io.WriteString(w, `{"access_token":"t","token_type":"bearer","expires_in":3600}`)
	case "/services/search":
		f.mu.Lock()
		fail := f.failServiceSearch
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"unavailable"}`)
			return
		}
		if strings.Contains(string(body), `"choreo-control-plane"`) {
			_, _ = io.WriteString(w, `{"services":[{"id":"svc-choreo","name":"choreo-control-plane","supportGroup":{"id":"aaaaaaaa-0000-4000-8000-000000000002","name":"SRE - Apollo"}}],"total":1}`)
			return
		}
		_, _ = io.WriteString(w, `{"services":[],"total":0}`)
	case "/incidents/search":
		f.mu.Lock()
		f.incidentSearches++
		f.mu.Unlock()
		_, _ = io.WriteString(w, `{"incidents":[],"total":0}`)
	case "/incidents":
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		// entity-service alone decides the assignment group and rejects a create that names one.
		if _, set := req["assignmentGroupId"]; set {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"message":"assignmentGroupId is not accepted"}`)
			return
		}
		f.mu.Lock()
		f.creates = append(f.creates, req)
		f.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"message":"created","incident":{"id":"inc-1","number":"INC0010001"}}`)
	default:
		http.NotFound(w, r)
	}
}

// newFakeCSMNotifier starts fake over TLS and returns a notifier wired to it through the real CSM client.
func newFakeCSMNotifier(t *testing.T, fake *fakeCSM) *notify.Notifier {
	t.Helper()
	srv := httptest.NewTLSServer(fake)
	// alert-core only speaks HTTPS to CSM; trust the test server's certificate for this run.
	saved := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() {
		srv.Client().CloseIdleConnections()
		http.DefaultTransport = saved
		srv.Close()
	})
	client := csm.NewClient(csm.Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
	return notify.New(testLogger(), client, notify.Config{
		CallerID:         "caller",
		DefaultServiceID: "svc-default",
		ServiceCacheTTL:  time.Minute,
		MaxAttempts:      1,
		RetryBaseDelay:   time.Millisecond,
		HTTPTimeout:      5 * time.Second,
	})
}

// An AWS alarm, as ingestion stores it, through the real engine, notifier and CSM client: the
// POST /incidents that reaches CSM names the alarm's service, or the Default service when it has none
// or an unknown one, and never an assignment group: entity-service sets that from the service.
func TestAWSAlarm_CreatesTheIncidentAgainstItsService(t *testing.T) {
	cases := []struct {
		alert, wantService string
	}{
		{"named", "svc-choreo"},      // a group the alarm names is recorded, not sent
		{"service", "svc-choreo"},    // the CMDB service
		{"unmatched", "svc-default"}, // a label with no CMDB match: the Default service
		{"account", "svc-default"},   // no service label: the Default service
	}
	for _, tc := range cases {
		t.Run(tc.alert, func(t *testing.T) {
			fake := &fakeCSM{}
			n := newFakeCSMNotifier(t, fake)

			var alert model.Alert
			if err := json.Unmarshal([]byte(awsStoredAlerts[tc.alert]), &alert); err != nil {
				t.Fatal(err)
			}
			e := New(testLogger(), newFakeIncidents(), n, testConfig())
			items := []Item{{ID: "ALT000000001", Alert: e.Normalize(alert)}}
			if err := e.HandleGroup(context.Background(), fpOf(items[0]), items); err != nil {
				t.Fatalf("HandleGroup: %v", err)
			}
			e.DeliverDue(context.Background())

			if len(fake.creates) != 1 {
				t.Fatalf("%d incident creates reached CSM, want 1", len(fake.creates))
			}
			req := fake.creates[0]
			if g, set := req["assignmentGroupId"]; set {
				t.Errorf("assignmentGroupId = %v; want it never sent", g)
			}
			if req["serviceId"] != tc.wantService {
				t.Errorf("serviceId = %v, want %s", req["serviceId"], tc.wantService)
			}
			if _, set := req["contactType"]; set {
				t.Errorf("contactType = %v; AWS has none in the enum", req["contactType"])
			}
			if req["impact"] != "HIGH" || req["urgency"] != "HIGH" || req["subject"] != "prod-rds-cpu-utilization-high" {
				t.Errorf("impact/urgency/subject = %v/%v/%v; want HIGH/HIGH and the alarm name", req["impact"], req["urgency"], req["subject"])
			}
		})
	}
}

// A failed service search is not "no match": the create waits for a retry rather than landing on the
// Default service, and the retry raises it against the real service.
func TestNotifyCSM_ServiceSearchErrorRetriesWithoutFallingBack(t *testing.T) {
	fake := &fakeCSM{failServiceSearch: true}
	n := newFakeCSMNotifier(t, fake)
	inc := model.Incident{
		IncidentNumber: "1", Fingerprint: "0123456789abcdef", FirstSeen: time.Now(),
		Service: "choreo-control-plane", Source: "AWS", Impact: "HIGH", Urgency: "HIGH", CSMAttempts: 1,
	}

	if _, _, ok, permanent := n.NotifyCSM(context.Background(), inc, ""); ok || permanent {
		t.Fatalf("NotifyCSM with a failing service search = ok %v, permanent %v; want a retryable failure", ok, permanent)
	}
	if len(fake.creates) != 0 {
		t.Fatalf("%d creates reached CSM while the service search was failing, want 0 (got serviceId %v)", len(fake.creates), fake.creates[0]["serviceId"])
	}

	fake.mu.Lock()
	fake.failServiceSearch = false
	fake.mu.Unlock()
	inc.CSMAttempts = 2
	if _, _, ok, _ := n.NotifyCSM(context.Background(), inc, ""); !ok {
		t.Fatal("NotifyCSM retry failed")
	}
	if len(fake.creates) != 1 || fake.creates[0]["serviceId"] != "svc-choreo" {
		t.Errorf("retry created %v, want one incident against svc-choreo", fake.creates)
	}
}

// A retry creates its incident straight away: CSM is not searched for a prior create, so an unrelated
// incident is never reused and a search that ignores its filter can no longer block the create.
func TestNotifyCSM_RetryCreatesWithoutSearching(t *testing.T) {
	fake := &fakeCSM{}
	n := newFakeCSMNotifier(t, fake)
	inc := model.Incident{
		IncidentNumber: "1", Fingerprint: "0123456789abcdef", FirstSeen: time.Now(),
		Service: "choreo-control-plane", Source: "AWS", Impact: "HIGH", Urgency: "HIGH", CSMAttempts: 2,
	}

	_, number, ok, _ := n.NotifyCSM(context.Background(), inc, "")
	if !ok || number != "INC0010001" {
		t.Fatalf("NotifyCSM = number %q, ok %v; want the newly created INC0010001", number, ok)
	}
	if len(fake.creates) != 1 || fake.incidentSearches != 0 {
		t.Errorf("creates %d, incident searches %d; want 1 create and no search", len(fake.creates), fake.incidentSearches)
	}
	if fake.creates[0]["correlationId"] != notify.CorrelationTag(inc.Fingerprint, inc.FirstSeen) {
		t.Errorf("correlationId = %v, want the incident's correlation tag", fake.creates[0]["correlationId"])
	}
}
