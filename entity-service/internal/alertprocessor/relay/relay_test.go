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

package relay

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

type fakePublisher struct {
	keys   []string
	failAt int
}

func (p *fakePublisher) Publish(_ context.Context, key, _ []byte) error {
	if len(p.keys) == p.failAt {
		return errors.New("event hub down")
	}
	p.keys = append(p.keys, string(key))
	return nil
}

func TestRelayStopsAtTheFirstFailure(t *testing.T) {
	pub := &fakePublisher{failAt: 2}
	r := &Relay{Publisher: pub}
	batch := []event.Event{alert("A1"), alert("A2"), alert("A3")}
	got := r.publish(context.Background(), batch)
	if !slices.Equal(got, []string{"A1", "A2"}) {
		t.Fatalf("published %v, want A1 and A2 only so A3 stays queued in order", got)
	}
	if pub.keys[0] != batch[0].Fingerprint() {
		t.Fatalf("key = %q, want the fingerprint so one fingerprint stays on one partition", pub.keys[0])
	}
}

func alert(id string) event.Event {
	return event.Event{AlertID: id, Alert: model.Alert{Source: "azure", Service: "svc", MetricName: "cpu"}}
}
