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

package publisher

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeProducer struct {
	keys   []string
	values [][]byte
	failAt int
}

func (p *fakeProducer) Publish(_ context.Context, key, value []byte) error {
	if len(p.keys) == p.failAt {
		return errors.New("event hub down")
	}
	p.keys, p.values = append(p.keys, string(key)), append(p.values, value)
	return nil
}

func row(id string) domain.NewAlert {
	return domain.NewAlert{Number: id, Tier: domain.AlertTierPriority,
		Alert: model.Alert{Source: "azure", Service: "svc", MetricName: "cpu", UniqueIdentifier: id}}
}

func TestPublishSkipsRowsWithNoAlertAndStopsAtTheFirstFailure(t *testing.T) {
	// Such as an error row stored before alerts had columns, returned again for a redelivered message.
	errorRow := domain.NewAlert{Number: "E1"}
	producer := &fakeProducer{failAt: -1}
	if err := (Publisher{Producer: producer}).Publish(context.Background(), []domain.NewAlert{row("A1"), errorRow, row("A2")}); err != nil {
		t.Fatal(err)
	}
	if len(producer.values) != 2 {
		t.Fatalf("published %d records, want the two alert rows and never the row with no alert", len(producer.values))
	}
	e, ok, err := event.Decode(producer.values[0])
	if err != nil || !ok || e.AlertID != "A1" || producer.keys[0] != e.Fingerprint() {
		t.Fatalf("first record = %+v (key %q), want A1 keyed by its fingerprint", e, producer.keys[0])
	}

	failing := &fakeProducer{failAt: 1}
	if err := (Publisher{Producer: failing}).Publish(context.Background(), []domain.NewAlert{row("A1"), row("A2"), row("A3")}); err == nil || len(failing.keys) != 1 {
		t.Fatalf("err=%v sent=%d, want the error after A1 so the message is redelivered", err, len(failing.keys))
	}
}

func TestRepublishRaisesTheAttemptUntilTheCap(t *testing.T) {
	producer := &fakeProducer{failAt: -1}
	p := Publisher{Producer: producer}
	e := event.Event{AlertID: "A1"}
	var attempts []int
	for {
		sent, err := p.Republish(context.Background(), e)
		if err != nil {
			t.Fatal(err)
		}
		if !sent {
			break
		}
		got, _, _ := event.Decode(producer.values[len(producer.values)-1])
		attempts = append(attempts, got.Attempt)
		e = got
	}
	if len(attempts) != MaxAttempts-1 || !slices.IsSorted(attempts) || attempts[len(attempts)-1] != MaxAttempts-1 {
		t.Fatalf("attempts %v, want 1..%d then no more", attempts, MaxAttempts-1)
	}
}
