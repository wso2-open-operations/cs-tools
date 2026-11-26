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

package subscriber

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
)

// fakeSource delivers fixed records once.
type fakeSource struct {
	records [][]byte
	errs    []error
}

func (f *fakeSource) Run(ctx context.Context, handle eventbus.Handle, _ eventbus.OnExhausted) {
	for _, v := range f.records {
		f.errs = append(f.errs, handle(ctx, eventbus.Record{Value: v}))
	}
}

func record(t *testing.T, id string) []byte {
	t.Helper()
	_, value, err := event.Encode(event.Event{AlertID: id, Alert: model.Alert{Source: "azure"}})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestSubscriberDeliversOnlyAlertCreated(t *testing.T) {
	src := &fakeSource{records: [][]byte{record(t, "A1"), []byte(`{"type":"incident.created","payload":{}}`), []byte(`garbage`), record(t, "A2")}}
	var got []string
	New(src, func(_ context.Context, e event.Event) error { got = append(got, e.AlertID); return nil }).Run(context.Background())
	if !slices.Equal(got, []string{"A1", "A2"}) {
		t.Fatalf("handled %v, want A1 and A2", got)
	}
	for i, err := range src.errs {
		if err != nil {
			t.Errorf("record %d: %v, want skipped records acknowledged", i, err)
		}
	}
}

func TestMiddlewareOrderAndRecover(t *testing.T) {
	var order []string
	tag := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, e event.Event) error {
				order = append(order, name)
				return next(ctx, e)
			}
		}
	}
	h := Wrap(func(context.Context, event.Event) error { panic("boom") }, tag("outer"), Recover(), tag("inner"))
	err := h(context.Background(), event.Event{AlertID: "A1"})
	if err == nil || !slices.Equal(order, []string{"outer", "inner"}) {
		t.Fatalf("err=%v order=%v, want the panic as an error and outer before inner", err, order)
	}
	want := errors.New("db down")
	if got := Wrap(func(context.Context, event.Event) error { return want }, Log())(context.Background(), event.Event{}); !errors.Is(got, want) {
		t.Fatalf("Log changed the error: %v", got)
	}
}
