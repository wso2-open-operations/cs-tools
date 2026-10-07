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

package dispatch

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

// On a shared topic an unknown type belongs to some other consumer: skipped
// cleanly, so it is neither retried nor dead-lettered. (This used sr.created
// as its example until this service started handling the sr.* types.)
func TestDispatcher_HandleShared_SkipsUnknownTypes(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
	rec := eventbus.Record{Topic: "sre-events", Value: []byte(`{"type":"problem.created","entityId":"PRB-1","payload":{}}`)}

	if err := d.HandleShared(context.Background(), rec); err != nil {
		t.Fatalf("unknown type on a shared topic returned %v, want nil", err)
	}
	if err := d.Handle(context.Background(), rec); err == nil {
		t.Error("Handle (an owned topic) must still reject an unknown type")
	}
	if len(mock.calls) != 0 {
		t.Errorf("sent %d emails for an unknown type", len(mock.calls))
	}
}

// A known type is handled exactly as on its own topic.
func TestDispatcher_HandleShared_HandlesKnownTypes(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	if err := d.HandleShared(context.Background(), outageRecord("outage.communication_due", "DECLARED", "Hello Team,")); err != nil {
		t.Fatalf("HandleShared: %v", err)
	}
	if err := d.HandleShared(context.Background(), crRecord("internal", "")); err != nil {
		t.Fatalf("HandleShared CR: %v", err)
	}
	if len(mock.calls) != 2 {
		t.Errorf("sent %d emails, want the outage and the CR notice", len(mock.calls))
	}
}

// A malformed record is still an error on a shared topic: it is broken, not
// someone else's.
func TestDispatcher_HandleShared_RejectsMalformed(t *testing.T) {
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{})
	if err := d.HandleShared(context.Background(), eventbus.Record{Value: []byte(`{not json`)}); err == nil {
		t.Error("a malformed record must still fail")
	}
}
