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

func TestTransform_GrafanaReceiverUsesTitle(t *testing.T) {
	body := []byte(`{"receiver":"grafana-default-email","status":"firing","alerts":[{"labels":{"alertname":"HighLatency"},"annotations":{"title":"Checkout latency"}}]}`)
	alerts, err := Transform(body, Config{})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(alerts) != 1 || alerts[0].MetricName != "Grafana Alert: Checkout latency" {
		t.Fatalf("alerts = %+v, want one alert named after the Grafana title", alerts)
	}
}
