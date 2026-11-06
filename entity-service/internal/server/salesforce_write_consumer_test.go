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

type fakeCompleter struct {
	completed []events.SalesforceMembershipWriteCompletedPayload
	failed    []events.SalesforceMembershipWriteFailedPayload
	lastFlags []bool
	err       error
}

func (f *fakeCompleter) CompleteDeferredSalesforceWrite(_ context.Context, p events.SalesforceMembershipWriteCompletedPayload, last bool) error {
	f.completed = append(f.completed, p)
	f.lastFlags = append(f.lastFlags, last)
	return f.err
}

func (f *fakeCompleter) FailDeferredSalesforceWrite(_ context.Context, p events.SalesforceMembershipWriteFailedPayload) error {
	f.failed = append(f.failed, p)
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

func TestSalesforceWriteResultHandle_CompletedCallsTheCompleter(t *testing.T) {
	c := &fakeCompleter{err: errors.New("sales entity unreadable")}
	body := envelope(t, events.TypeSalesforceMembershipWriteCompleted, "req-1",
		events.SalesforceMembershipWriteCompletedPayload{RequestID: "req-1", ContactSfID: "003", MembershipSfID: "a0e"})

	err := salesforceWriteResultHandle(c)(context.Background(), eventbus.Record{Value: body, NoMoreRetries: true})
	if err == nil || err.Error() != "sales entity unreadable" {
		t.Fatalf("the completer's error must propagate so the consumer retries; got %v", err)
	}
	if len(c.completed) != 1 || c.completed[0].MembershipSfID != "a0e" || !c.lastFlags[0] {
		t.Errorf("completed calls = %+v last=%v", c.completed, c.lastFlags)
	}
}

func TestSalesforceWriteResultHandle_FailedCallsTheCompleter(t *testing.T) {
	c := &fakeCompleter{}
	body := envelope(t, events.TypeSalesforceMembershipWriteFailed, "req-2", map[string]any{"reason": "refused", "terminal": true})
	if err := salesforceWriteResultHandle(c)(context.Background(), eventbus.Record{Value: body}); err != nil {
		t.Fatal(err)
	}
	if len(c.failed) != 1 || c.failed[0].RequestID != "req-2" || c.failed[0].Reason != "refused" || !c.failed[0].Terminal {
		t.Errorf("failed calls = %+v (request id must fall back to the entity id)", c.failed)
	}
}

// Anything this consumer cannot act on is acknowledged, not retried.
func TestSalesforceWriteResultHandle_SkipsWhatItCannotAct(t *testing.T) {
	cases := map[string][]byte{
		"malformed envelope": []byte("{not json"),
		"other event type":   envelope(t, events.TypeSalesforceMembershipWriteRequested, "x", map[string]any{}),
		"no request id":      envelope(t, events.TypeSalesforceMembershipWriteCompleted, "", map[string]any{}),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			c := &fakeCompleter{err: errors.New("must not be called")}
			if err := salesforceWriteResultHandle(c)(context.Background(), eventbus.Record{Value: body}); err != nil {
				t.Fatalf("want nil (acknowledge), got %v", err)
			}
			if len(c.completed)+len(c.failed) != 0 {
				t.Errorf("completer called for %s", name)
			}
		})
	}
}
