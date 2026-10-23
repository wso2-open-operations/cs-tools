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

package notify_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhours"
)

// End-to-end send harness for the two query-hour mails. Renders what
// entity-service actually produced (see QUERY_HOUR_REPORT_JSON_OUT in
// its query_hour_integration_test.go) and sends through the real
// notification service, so the whole chain is exercised at once.
//
// Skipped unless QUERY_HOUR_E2E_TO is set, and it must be an address
// the runner owns. The derived audience is logged and discarded; this
// never mails an account owner.
//
//	QUERY_HOUR_E2E_TO=you@example.com \
//	QUERY_HOUR_REPORT_JSON=/path/report.json \
//	QUERY_HOUR_TRANSITIONS_JSON=/path/transitions.json \
//	OAUTH2_CLIENT_ID=... OAUTH2_CLIENT_SECRET=... OAUTH2_TOKEN_URL=... \
//	EMAIL_BASE_URL=... EMAIL_FROM_ADDRESS=... \
//	  go test -run TestQueryHoursE2ESend ./internal/notify/
func TestQueryHoursE2ESend(t *testing.T) {
	to := os.Getenv("QUERY_HOUR_E2E_TO")
	if to == "" {
		t.Skip("QUERY_HOUR_E2E_TO is not set; skipping the live send")
	}

	client, err := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       strings.Fields(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		t.Fatalf("notify client: %v", err)
	}
	send := func(subject, body string) {
		t.Helper()
		if strings.Contains(body, "<!-- [") {
			t.Fatalf("unsubstituted placeholder in %q", subject)
		}
		if err := client.SendEmail(context.Background(), []string{to}, nil, subject, body); err != nil {
			t.Fatalf("send %q: %v", subject, err)
		}
		t.Logf("sent %q to %s (%d bytes)", subject, to, len(body))
	}

	if path := os.Getenv("QUERY_HOUR_REPORT_JSON"); path != "" {
		var report queryhours.Report
		decodeInto(t, path, &report)
		t.Logf("derived audience (NOT used): %v", report.Recipients)
		send(notify.QueryHoursWeeklySubject, notify.RenderQueryHoursWeekly(report))
	}

	if path := os.Getenv("QUERY_HOUR_TRANSITIONS_JSON"); path != "" {
		var payload struct {
			Transitions []queryhours.Transition `json:"transitions"`
		}
		decodeInto(t, path, &payload)
		for _, tr := range payload.Transitions {
			t.Logf("would notify %s / %s (NOT used)", tr.AccountManagerEmail, tr.TechnicalOwnerEmail)
			send(notify.QueryHourNotificationSubject(tr), notify.RenderQueryHourNotification(tr))
		}
	}
}

func decodeInto(t *testing.T, path string, into any) {
	t.Helper()
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(blob, into); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
