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

package prometheus

import "testing"

// A Grafana alert found by its receiver alone keeps alertname as its metric name, as alert-ingestion does, so the fingerprint matches; only grafana_folder switches to the title.
func TestTransform_GrafanaMetricNameMatchesAlertIngestion(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"receiver only":  {`{"receiver":"grafana-default-email","status":"firing","alerts":[{"labels":{"alertname":"HighLatency"},"annotations":{"title":"Checkout latency"}}]}`, "HighLatency"},
		"grafana_folder": {`{"receiver":"other","status":"firing","alerts":[{"labels":{"alertname":"HighLatency","grafana_folder":"apps"},"annotations":{"title":"Checkout latency"}}]}`, "Grafana Alert: Checkout latency"},
	} {
		t.Run(name, func(t *testing.T) {
			alerts, err := Transform([]byte(tc.body), Config{})
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			if len(alerts) != 1 || alerts[0].MetricName != tc.want || alerts[0].Source != "Grafana" {
				t.Fatalf("alerts = %+v, want one Grafana alert named %q", alerts, tc.want)
			}
		})
	}
}
