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

package main

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/paging"
)

func TestSREIncidentConsumer(t *testing.T) {
	for name, tc := range map[string]struct {
		kind                    paging.Ladder
		incidentTopic, override string
		wantTopic, wantGroup    string
		wantOK                  bool
	}{
		"unset: nothing extra":             {paging.LadderSRE, "", "", "", "", false},
		"same as the shared topic":         {paging.LadderSRE, "case-events", "", "", "", false},
		"SRE ladder reads sre-events":      {paging.LadderSRE, " sre-events ", "", "sre-events", "csm-notification-service-paging-sre-incidents", true},
		"group override":                   {paging.LadderSRE, "sre-events", " my-group ", "sre-events", "my-group", true},
		"CRE ladder never reads incidents": {paging.LadderCRE, "sre-events", "", "", "", false},
	} {
		topic, group, ok := sreIncidentConsumer(tc.kind, "case-events", tc.incidentTopic, tc.override)
		if ok != tc.wantOK || topic != tc.wantTopic || group != tc.wantGroup {
			t.Errorf("%s: (%q, %q, %v), want (%q, %q, %v)", name, topic, group, ok, tc.wantTopic, tc.wantGroup, tc.wantOK)
		}
	}
}

func TestIncidentDispatchConsumer(t *testing.T) {
	const mainGroup = "csm-notification-service"
	sreOn := srePlan{Enabled: true, SRE: consumerTarget{"sre-events", "g"}}
	for name, tc := range map[string]struct {
		incidentTopic, override string
		plan                    srePlan
		wantTopic, wantGroup    string
		wantOK                  bool
	}{
		"unset: nothing extra":                  {"", "", srePlan{}, "", "", false},
		"same as the shared topic":              {"case-events", "", srePlan{}, "", "", false},
		"sre-events consumer already reads it":  {"sre-events", "", sreOn, "", "", false},
		"sre-events consumer off: own consumer": {" sre-events ", "", srePlan{}, "sre-events", mainGroup + "-incidents", true},
		"sre-events consumer on another topic":  {"incident-events", "", sreOn, "incident-events", mainGroup + "-incidents", true},
		"group override":                        {"sre-events", " my-group ", srePlan{}, "sre-events", "my-group", true},
	} {
		topic, group, ok := incidentDispatchConsumer("case-events", tc.incidentTopic, tc.override, mainGroup, tc.plan)
		if ok != tc.wantOK || topic != tc.wantTopic || group != tc.wantGroup {
			t.Errorf("%s: (%q, %q, %v), want (%q, %q, %v)", name, topic, group, ok, tc.wantTopic, tc.wantGroup, tc.wantOK)
		}
	}
}
