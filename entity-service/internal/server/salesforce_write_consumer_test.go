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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

type fakeDeferredWriter struct {
	calls []struct {
		id   string
		last bool
	}
	err error
}

func (f *fakeDeferredWriter) ProcessDeferredSalesforceWrite(_ context.Context, requestID string, lastAttempt bool) error {
	f.calls = append(f.calls, struct {
		id   string
		last bool
	}{requestID, lastAttempt})
	return f.err
}

func envelope(t *testing.T, typ events.Type, entityID string, payload any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(events.Envelope{Type: typ, EntityID: entityID, Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSalesforceWriteHandle_CallsTheWriterWithTheRequestID(t *testing.T) {
	w := &fakeDeferredWriter{err: errors.New("salesforce down")}
	handle := salesforceWriteHandle(w)
	body := envelope(t, events.TypeSalesforceMembershipWriteRequested, "req-1",
		events.SalesforceMembershipWriteRequestedPayload{RequestID: "req-1", Operation: "INVITE"})

	err := handle(context.Background(), eventbus.Record{Value: body, NoMoreRetries: true})
	if err == nil || err.Error() != "salesforce down" {
		t.Fatalf("the writer's error must propagate so the consumer retries; got %v", err)
	}
	if len(w.calls) != 1 || w.calls[0].id != "req-1" || !w.calls[0].last {
		t.Errorf("calls = %+v, want one call for req-1 with lastAttempt", w.calls)
	}
}

func TestSalesforceWriteHandle_FallsBackToTheEntityID(t *testing.T) {
	w := &fakeDeferredWriter{}
	body := envelope(t, events.TypeSalesforceMembershipWriteRequested, "req-2", map[string]any{})
	if err := salesforceWriteHandle(w)(context.Background(), eventbus.Record{Value: body}); err != nil {
		t.Fatal(err)
	}
	if len(w.calls) != 1 || w.calls[0].id != "req-2" || w.calls[0].last {
		t.Errorf("calls = %+v", w.calls)
	}
}

// Anything this service did not publish is acknowledged, not retried: there
// is nothing a retry or a dead-letter could do with it.
func TestSalesforceWriteHandle_SkipsWhatItCannotAct(t *testing.T) {
	cases := map[string][]byte{
		"malformed envelope": []byte("{not json"),
		"other event type":   envelope(t, events.TypeProjectContactInvited, "x", map[string]any{}),
		"no request id":      envelope(t, events.TypeSalesforceMembershipWriteRequested, "", map[string]any{}),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := &fakeDeferredWriter{err: errors.New("must not be called")}
			if err := salesforceWriteHandle(w)(context.Background(), eventbus.Record{Value: body}); err != nil {
				t.Fatalf("want nil (acknowledge), got %v", err)
			}
			if len(w.calls) != 0 {
				t.Errorf("writer called for %s: %+v", name, w.calls)
			}
		})
	}
}
