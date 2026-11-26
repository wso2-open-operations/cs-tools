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

package event

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestRoundTrip(t *testing.T) {
	e := Event{AlertID: "A1", Tier: domain.AlertTierPriority, ReceivedOn: time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC),
		Alert: model.Alert{Source: "azure", Service: "svc", MetricName: "cpu", Severity: "critical"}}
	key, value, err := Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != e.Fingerprint() {
		t.Fatalf("key = %q, want the fingerprint so one fingerprint stays on one partition", key)
	}
	got, ok, err := Decode(value)
	if err != nil || !ok || got.AlertID != "A1" || got.Alert != e.Alert || !got.ReceivedOn.Equal(e.ReceivedOn) || got.Tier != e.Tier {
		t.Fatalf("Decode = %+v ok=%v err=%v, want the event back", got, ok, err)
	}
}

func TestDecodeSkipsOtherEventTypes(t *testing.T) {
	if _, ok, err := Decode([]byte(`{"type":"incident.created","entityId":"x","payload":{}}`)); ok || err != nil {
		t.Fatalf("ok=%v err=%v, want another event type skipped without an error", ok, err)
	}
	if _, _, err := Decode([]byte(`not json`)); err == nil {
		t.Fatal("Decode accepted a malformed record")
	}
	if _, _, err := Decode([]byte(`{"type":"alert.created","payload":{}}`)); err == nil {
		t.Fatal("Decode accepted an alert.created with no alert id")
	}
}
