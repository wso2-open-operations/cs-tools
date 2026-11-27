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

package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

func TestAlertProcessorModesUseSeparateGroups(t *testing.T) {
	if got := consumerGroup("alerts", true); got != "alerts" {
		t.Errorf("v2 group = %q, want the configured group", got)
	}
	if got := consumerGroup("alerts", false); got != "alerts-hybrid" {
		t.Errorf("hybrid group = %q, want its own group so it never commits v2's offsets", got)
	}
	if !subscriberBus(eventbus.Config{}, true).StartAtLatest || subscriberBus(eventbus.Config{}, false).StartAtLatest {
		t.Error("want only v2 to start at the newest event, so switching does not replay alerts v1 handled")
	}
}

// fakeFailures records event_publish_failures rows; the embedded interface panics on any other method.
type fakeFailures struct {
	service.EventPublishFailureService
	created []domain.CreateEventPublishFailureRequest
	err     error
}

func (f *fakeFailures) CreateEventPublishFailure(_ context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error) {
	f.created = append(f.created, req)
	return domain.EventPublishFailure{}, f.err
}

func TestRecordAlertFailureKeepsTheEventForReplay(t *testing.T) {
	failures := &fakeFailures{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // at shutdown the record must still be written
	if err := recordAlertFailure(ctx, failures, event.Event{AlertID: "ALT000000001"}, "could not publish again"); err != nil {
		t.Fatal(err)
	}
	if len(failures.created) != 1 {
		t.Fatalf("recorded %d rows, want 1", len(failures.created))
	}
	got := failures.created[0]
	decoded, ok, err := event.Decode(got.Payload)
	if got.EventType != "alert.created" || got.EntityID != "ALT000000001" || err != nil || !ok || decoded.AlertID != "ALT000000001" || !strings.Contains(got.Error, "could not publish") {
		t.Fatalf("row = %+v, want the alert.created record that can be replayed as is", got)
	}
	failures.err = errors.New("database down")
	if err := recordAlertFailure(context.Background(), failures, event.Event{AlertID: "ALT000000002"}, "x"); err == nil {
		t.Fatal("want the error when the record cannot be written")
	}
}
