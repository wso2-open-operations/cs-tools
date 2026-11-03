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
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

type fakeMailPager struct {
	got []events.MailAlertReceivedPayload
}

func (f *fakeMailPager) HandleAlert(_ context.Context, p events.MailAlertReceivedPayload) error {
	f.got = append(f.got, p)
	return nil
}

var mailAlertRecord = eventbus.Record{Value: []byte(`{"type":"mail.alert.received","entityId":"m1","payload":` +
	`{"ruleName":"sre","messageId":"m1","dedupeKey":"m1/sre","subject":"Disk full","from":"alerts@example.com"}}`)}

// mail.alert.received reaches the pager; with none configured it is
// acknowledged, not retried.
func TestDispatcher_Handle_MailAlert(t *testing.T) {
	mp := &fakeMailPager{}
	d := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).WithMailAlertPaging(mp)
	if err := d.Handle(context.Background(), mailAlertRecord); err != nil {
		t.Fatal(err)
	}
	if len(mp.got) != 1 || mp.got[0].RuleName != "sre" || mp.got[0].DedupeKey != "m1/sre" {
		t.Fatalf("pager got %+v", mp.got)
	}
	if err := newTestDispatcher(&mockEmailSender{}, &mockGoogleChatSender{}, &mockCallSender{}).Handle(context.Background(), mailAlertRecord); err != nil {
		t.Errorf("without a pager: %v; want the event acknowledged", err)
	}
}
