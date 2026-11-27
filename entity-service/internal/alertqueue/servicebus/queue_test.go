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

package servicebus

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestMessage_ReadsTheServiceBusFields(t *testing.T) {
	ct := "application/json"
	at := time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)
	m := message{&azservicebus.ReceivedMessage{
		MessageID: "m1", Body: []byte("{}"), ContentType: &ct, EnqueuedTime: &at,
		ApplicationProperties: map[string]any{"source": "azure"},
	}}
	if m.ID() != "m1" || string(m.Body()) != "{}" || m.ContentType() != ct || m.Source() != "azure" || !m.EnqueuedOn().Equal(at) {
		t.Fatalf("message = %s %q %q %q %s, want the Service Bus fields", m.ID(), m.Body(), m.ContentType(), m.Source(), m.EnqueuedOn())
	}
	bare := message{&azservicebus.ReceivedMessage{MessageID: "m2", ApplicationProperties: map[string]any{"source": 7}}}
	if bare.ContentType() != "" || bare.Source() != "7" || !bare.EnqueuedOn().IsZero() {
		t.Fatalf("bare message = %q %q %s, want empty content type, the source as text, no time", bare.ContentType(), bare.Source(), bare.EnqueuedOn())
	}
	if (message{&azservicebus.ReceivedMessage{}}).Source() != "" {
		t.Fatal("a message with no source property has a source")
	}
}

func TestNewClient_NeverQuotesTheConnectionString(t *testing.T) {
	_, err := NewClient("Endpoint=nope;SharedAccessKey=c2VjcmV0")
	if err == nil {
		t.Skip("the SDK accepted the string; nothing to check")
	}
	if got := err.Error(); got != "service bus: the connection string is not valid" {
		t.Fatalf("error = %q, want the generic message only", got)
	}
}
