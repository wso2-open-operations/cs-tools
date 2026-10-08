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
	"named":   `{"service":"choreo-control-plane",` + awsAlertBase + `,"assignment_group":"SRE - Artemis"}`,
	"service": `{"service":"choreo-control-plane",` + awsAlertBase + `}`,
	"account": `{"service":"",` + awsAlertBase + `}`,
}

// fakeCSM is csm-integration-service as alert-core sees it: an OAuth2 token endpoint, the CMDB
// service search, the dedup search, and the incident create, whose request bodies it keeps.
type fakeCSM struct {
	mu      sync.Mutex
	creates []map[string]any
}

func (f *fakeCSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/token":
		_, _ = io.WriteString(w, `{"access_token":"t","token_type":"bearer","expires_in":3600}`)
	case "/services/search":
		if strings.Contains(string(body), `"choreo-control-plane"`) {
			_, _ = io.WriteString(w, `{"services":[{"id":"svc-choreo","name":"choreo-control-plane","supportGroup":{"id":"aaaaaaaa-0000-4000-8000-000000000002","name":"SRE - Apollo"}}],"total":1}`)
			return
		}
		_, _ = io.WriteString(w, `{"services":[],"total":0}`)
	case "/incidents/search":
		_, _ = io.WriteString(w, `{"incidents":[],"total":0}`)
	case "/incidents":
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		// entity-service takes the group from the service and rejects a create that names one, as an
		// unknown field (its decoder disallows unknown fields), which alert-core treats as permanent.
		if _, named := req["assignmentGroupId"]; named {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"code":400,"message":"unknown field \"assignmentGroupId\" in request body"}`)
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

// An AWS alarm, as ingestion stores it, through the real engine, notifier and CSM client, creates the
// incident in CSM whatever the routing chain resolves: the create names the service and never a group,
// so entity-service (which rejects a named group) accepts it and assigns the service's support group.
func TestAWSAlarm_CreatesTheIncident(t *testing.T) {
	cases := []struct {
		alert, wantService string
	}{
		{"named", "svc-choreo"},    // the alarm names its own group
		{"service", "svc-choreo"},  // the CMDB service has a support group
		{"account", "svc-unknown"}, // no service: only an account route matches
	}
	for _, tc := range cases {
		t.Run(tc.alert, func(t *testing.T) {
			fake := &fakeCSM{}
			srv := httptest.NewTLSServer(fake)
			defer srv.Close()
			// alert-core only speaks HTTPS to CSM; trust the test server's certificate for this run.
			saved := http.DefaultTransport
			http.DefaultTransport = srv.Client().Transport
			defer func() {
				srv.Client().CloseIdleConnections()
				http.DefaultTransport = saved
			}()

			client := csm.NewClient(csm.Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
			n := notify.New(testLogger(), client, notify.Config{
				CallerID:         "caller",
				UnknownServiceID: "svc-unknown",
				ServiceCacheTTL:  time.Minute,
				MaxAttempts:      1,
				RetryBaseDelay:   time.Millisecond,
				HTTPTimeout:      5 * time.Second,
				AssignmentGroupRoutes: map[string]string{
					"group:SRE - Artemis":  "aaaaaaaa-0000-4000-8000-000000000001",
					"account:487629103847": "aaaaaaaa-0000-4000-8000-000000000004",
				},
				DefaultAssignmentGroupID: "aaaaaaaa-0000-4000-8000-000000000005",
			})

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
			if v, named := req["assignmentGroupId"]; named {
				t.Errorf("assignmentGroupId = %v; want it omitted", v)
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
