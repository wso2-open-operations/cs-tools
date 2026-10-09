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

package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
)

// alert-core never names an assignment group: entity-service sets it from the service (the Default
// service's support group when the alert has none), and rejects a create that names one.
func TestCreateRequest_CarriesServiceAndContactTypeButNoGroup(t *testing.T) {
	n := &Notifier{callerID: "caller"}
	inc := model.Incident{Fingerprint: "fp", Service: "svc", Source: "Azure", Category: "availability", Impact: "HIGH", Urgency: "HIGH", AssignmentGroup: "SRE - Apollo"}

	got := createBody(t, n.createRequest(inc, resolvedService{id: "svc-id"}, "[fp:tag]", "Incident auto-created from Alert: ALT1"))
	if v, present := got["assignmentGroupId"]; present {
		t.Errorf("assignmentGroupId = %v; want it never sent", v)
	}
	if got["contactType"] != "AZURE" {
		t.Errorf("contactType = %v, want AZURE", got["contactType"])
	}
	if got["serviceId"] != "svc-id" {
		t.Errorf("serviceId = %v, want svc-id", got["serviceId"])
	}
}

// AWS (and any source the contact-type enum has no value for) sends no contactType rather than a wrong one.
func TestCreateRequest_OmitsWhatIsUnknown(t *testing.T) {
	n := &Notifier{callerID: "caller"}
	got := createBody(t, n.createRequest(model.Incident{Fingerprint: "fp", Source: "AWS"}, resolvedService{id: "svc-id"}, "[fp:tag]", ""))
	for _, key := range []string{"assignmentGroupId", "contactType"} {
		if v, present := got[key]; present {
			t.Errorf("%s = %v; want it omitted", key, v)
		}
	}
}

// An alert with no Service label is raised against the Default service without searching CSM.
func TestResolveService_EmptyLabelUsesTheDefaultService(t *testing.T) {
	n := New(testLogger(), nil, Config{DefaultServiceID: "svc-default", ServiceCacheTTL: time.Minute})
	svc, err := n.resolveService(context.Background(), "")
	if err != nil || svc.id != "svc-default" {
		t.Errorf("resolveService(\"\") = (%q, %v), want (svc-default, nil)", svc.id, err)
	}
}

// createBody is the JSON body the CSM client would send for req, decoded into a map.
func createBody(t *testing.T, req csm.CreateIncidentRequest) map[string]any {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestContactTypeForSource(t *testing.T) {
	for source, want := range map[string]string{
		"Azure":              "AZURE",
		"azure-monitor":      "AZURE",
		"Site24x7":           "SITE_247",
		"Site 24x7":          "SITE_247",
		"Microsoft Sentinel": "SENTINEL",
		"AWS":                "",
		"Grafana":            "",
		"":                   "",
	} {
		if got := contactTypeForSource(source); got != want {
			t.Errorf("contactTypeForSource(%q) = %q, want %q", source, got, want)
		}
	}
}
