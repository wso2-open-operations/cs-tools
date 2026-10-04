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

package outagecommtask_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecomm"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecommtask"
)

// A LIVE delivery run: the real client, the real email service, the real task.
//
// Skipped unless OUTAGE_COMM_LIVE_BASE_URL is set, so an ordinary `go test`
// never sends mail.
//
// *** THE RECIPIENT IS PINNED TO ONE ADDRESS AND VERIFIED AT RUN TIME. ***
// This test mails a real inbox. The address is not taken from the
// environment, and the guard below refuses to run if it is ever edited to
// anything else -- a test that sends to a standing group by accident is
// exactly the failure this whole port exists to avoid. The committed value
// is a placeholder; whoever runs this edits it locally to their own single
// address (both places below) and does not commit that edit.
const liveTestRecipient = "jane.doe@example.com"

func TestLiveSend(t *testing.T) {
	base := os.Getenv("OUTAGE_COMM_LIVE_BASE_URL")
	if base == "" {
		t.Skip("OUTAGE_COMM_LIVE_BASE_URL not set; this run sends real email")
	}

	// Belt and braces. If someone widens the recipient, fail loudly here
	// rather than in someone's inbox.
	if liveTestRecipient != "jane.doe@example.com" || strings.Contains(liveTestRecipient, ",") {
		t.Fatalf("refusing to run: recipient is %q, expected exactly jane.doe@example.com", liveTestRecipient)
	}

	sweeper, err := outagecomm.NewClient(outagecomm.Config{
		BaseURL:      base,
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
	})
	if err != nil {
		t.Fatalf("outagecomm client: %v", err)
	}

	email, err := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		t.Fatalf("notify client: %v", err)
	}

	handler := outagecommtask.SendCommunications(
		sweeper, email,
		[]string{liveTestRecipient}, // to  -- one address, nobody else
		nil,                         // cc  -- deliberately empty
		true,                        // ALERTS_ENABLED
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := handler(ctx); err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("handler returned cleanly; mail sent to %s only", liveTestRecipient)
}
