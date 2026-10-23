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
	"os"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
)

// End-to-end send harness for the monthly product regression report mail:
// renders a sample report and mails it through the real notification
// service, so the whole chain is exercised once and the layout can be
// checked in a real inbox.
//
// Skipped unless REGRESSION_E2E_TO is set, and it must be a real address
// the runner owns.
//
//	REGRESSION_E2E_TO=you@example.com \
//	PRODUCT_REGRESSION_REPORT_LINK_BASE_URL=https://.../cases \
//	OAUTH2_CLIENT_ID=... OAUTH2_CLIENT_SECRET=... OAUTH2_TOKEN_URL=... \
//	EMAIL_BASE_URL=... EMAIL_FROM_ADDRESS=... EMAIL_SCOPES=... \
//	  go test -run TestProductRegressionReportE2ESend ./internal/notify/
func TestProductRegressionReportE2ESend(t *testing.T) {
	to := os.Getenv("REGRESSION_E2E_TO")
	if to == "" {
		t.Skip("REGRESSION_E2E_TO is not set; skipping the live send")
	}
	loc, _ := time.LoadLocation("Asia/Colombo")
	report := regressionreport.Report{
		GeneratedAt: time.Now(),
		Month:       "2026-09",
		PeriodStart: time.Date(2026, 9, 1, 0, 0, 0, 0, loc),
		PeriodEnd:   time.Date(2026, 10, 1, 0, 0, 0, 0, loc),
		Tag:         "s_rg",
		Products:    []string{"WSO2 Identity Server"},
		TotalCases:  2,
		Groups: []regressionreport.Group{{
			Product: "WSO2 Identity Server",
			Cases: []regressionreport.Row{
				{Number: "CS0000001", IssueKey: "EXAMPLESUB-933", Subject: "[IS 7.1.0] Error while executing migration client",
					Product: "WSO2 Identity Server", DeployedProduct: "Identity Server 7.1.0 All In One",
					EtaSharedOn: time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)},
				{Number: "CS0000002", IssueKey: "EXAMPLESUB-67", Subject: "removing @carbon.super from UI",
					Product: "WSO2 Identity Server", DeployedProduct: "Identity Server 7.1.0 All In One",
					EtaSharedOn: time.Date(2026, 9, 20, 4, 0, 0, 0, time.UTC)},
			},
		}},
	}
	body := notify.RenderProductRegressionReport(report, "IAM", os.Getenv("PRODUCT_REGRESSION_REPORT_LINK_BASE_URL"))
	if out := os.Getenv("REGRESSION_E2E_HTML_OUT"); out != "" {
		if err := os.WriteFile(out, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
	}
	client, err := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		Scopes:       splitScopes(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		t.Fatalf("notify client: %v", err)
	}
	subject := "[Test copy] Monthly Product Regression Report - IAM"
	if err := client.SendEmail(context.Background(), []string{to}, nil, subject, body); err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("sent %q to %s: %d cases", subject, to, report.TotalCases)
}
