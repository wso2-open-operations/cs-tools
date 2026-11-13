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

package alertsource

import (
	"strings"
	"testing"
)

// TestDescription_IsReadableTextNotThePayload: every source fills description from the vendor's own message field and never with the request body, which raw_alerts keeps instead.
func TestDescription_IsReadableTextNotThePayload(t *testing.T) {
	t.Setenv("SITE24X7_ALERT_CONFIG", `{"TagList":{"Service":"svc","Category":"cat","Environment":"env"}}`)
	r, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ body, want string }{
		"aws": {`{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"{\"AlarmName\":\"cpu\",\"AlarmArn\":\"arn:a\",\"NewStateValue\":\"ALARM\",\"NewStateReason\":\"Threshold Crossed: 92.4 > 90\"}"}`,
			"Threshold Crossed: 92.4 > 90"},
		"aws plain message": {`{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"disk almost full"}`,
			"disk almost full"},
		"azure": {`{"schemaId":"azureMonitorCommonAlertSchema","data":{"essentials":{"alertId":"a1","alertRule":"rt","severity":"Sev1","monitorCondition":"Fired","description":"Response time above 2s"}}}`,
			"Response time above 2s"},
		"datadog": {`{"event_name":"mem","alert_id":"1","transition":"Triggered","body":"Memory usable below 10%"}`,
			"Memory usable below 10%"},
		"elasticsearch": {`{"rule_id":"r1","rule_name":"disk","trigger_name":"t","state":"ACTIVE","alert_id":"a1","message":"Disk watermark exceeded"}`,
			"Disk watermark exceeded"},
		"gcp": {`{"incident":{"incident_id":"i1","state":"open","policy_name":"p","summary":"5xx rate above 5%"},"version":"1.2"}`,
			"5xx rate above 5%"},
		"icinga": {`{"notification_type":"PROBLEM","host_name":"db1","service_name":"lag","service_state":"CRITICAL","service_output":"Replication lag 120s"}`,
			"Replication lag 120s"},
		"openobserve": {`{"short_description":"p99 high","description":"p99 above 2000ms for 5 minutes","urgency":"1","correlation_id":"c1"}`,
			"p99 above 2000ms for 5 minutes"},
		"opensearch": {`{"monitor_id":"m1","monitor_name":"heap","trigger_name":"t","state":"ACTIVE","alert_id":"a1","message":"JVM heap above 90%"}`,
			"JVM heap above 90%"},
		"prometheus": {`{"receiver":"r","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"Crash"},"annotations":{"summary":"Pod restarted 5 times"},"fingerprint":"f1"}]}`,
			"Pod restarted 5 times"},
		"site24x7": {`{"STATUS":"DOWN","MONITORNAME":"api","MONITOR_ID":"1","INCIDENT_REASON":"Connection timed out"}`,
			"Connection timed out"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			source, _, _ := strings.Cut(name, " ")
			transform, ok := r.Lookup(source)
			if !ok {
				t.Fatalf("no transform for %s", source)
			}
			alerts, err := transform([]byte(tc.body))
			if err != nil || len(alerts) != 1 {
				t.Fatalf("transform = %v, %v", alerts, err)
			}
			if got := alerts[0].Description; got != tc.want {
				t.Errorf("Description = %q, want %q", got, tc.want)
			}
		})
	}
}
