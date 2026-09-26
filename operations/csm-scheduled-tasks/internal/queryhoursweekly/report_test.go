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

package queryhoursweekly

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

type fakeReports struct {
	report queryhoursreport.Report
	err    error
	calls  int
}

func (f *fakeReports) WeeklyReport(ctx context.Context) (queryhoursreport.Report, error) {
	f.calls++
	return f.report, f.err
}

type fakeEmail struct {
	to, cc  []string
	subject string
	body    string
	calls   int
	err     error
}

func (f *fakeEmail) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return f.err
}

func sampleReport() queryhoursreport.Report {
	return queryhoursreport.Report{
		GeneratedOn:        "2026-09-20",
		ExceededCount:      1,
		GoingToExceedCount: 0,
		Exceeded: []queryhoursreport.Account{{
			Name:     "Intrepid Travel",
			SFID:     "acct-sf",
			RowCount: 1,
			Exceeded: true,
			Groups: []queryhoursreport.Group{{
				EntitlementMinutes: 6000,
				ConsumedMinutes:    6275,
				RemainingMinutes:   -275,
				Exceeded:           true,
				RowCount:           1,
				Opportunities: []queryhoursreport.Opportunity{{
					Name:               "Integration #WSO2CONPI",
					SFID:               "opp-sf",
					EntitlementMinutes: 6000,
					Projects: []queryhoursreport.Project{{
						Name:            "Intrepidsub - Subscription",
						Key:             "INTREPIDSUBSUB",
						SFID:            "proj-sf",
						ConsumedMinutes: 6275,
					}},
				}},
			}},
		}},
	}
}

// ALERTS_ENABLED=false must skip the fetch, not fetch and discard.
func TestSendReport_DisabledSkipsTheFetchEntirely(t *testing.T) {
	reports, email := &fakeReports{report: sampleReport()}, &fakeEmail{}
	err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "", false)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reports.calls != 0 {
		t.Errorf("report was fetched despite emails being disabled (%d calls)", reports.calls)
	}
	if email.calls != 0 {
		t.Errorf("email was sent despite being disabled")
	}
}

// No recipients means no consumer for the work, so it does none.
func TestSendReport_NoRecipientsSkipsTheFetchEntirely(t *testing.T) {
	reports, email := &fakeReports{report: sampleReport()}, &fakeEmail{}
	err := SendReport(reports, email, nil, []string{"cc@wso2.com"}, "", true)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("did work with nobody to send it to: fetches=%d sends=%d", reports.calls, email.calls)
	}
}

// An empty report is still sent: a silent week is indistinguishable from a
// broken job.
func TestSendReport_EmptyReportIsStillSent(t *testing.T) {
	reports := &fakeReports{report: queryhoursreport.Report{GeneratedOn: "2026-09-20"}}
	email := &fakeEmail{}
	err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "", true)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email.calls != 1 {
		t.Fatalf("expected the empty report to be sent, got %d sends", email.calls)
	}
	if !strings.Contains(email.body, "No accounts in this category") {
		t.Error("an empty report should say so in both tables")
	}
}

// The audience is the configured one. The account manager and technical owner
// entity-service reports must NOT leak into the recipients.
func TestSendReport_AddressesFromConfigNotFromTheData(t *testing.T) {
	report := sampleReport()
	report.Exceeded[0].AccountManagerEmail = "am@wso2.com"
	report.Exceeded[0].TechnicalOwnerEmail = "to@wso2.com"

	reports, email := &fakeReports{report: report}, &fakeEmail{}
	err := SendReport(reports, email, []string{"configured@wso2.com"}, []string{"cc@wso2.com"}, "", true)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(email.to) != 1 || email.to[0] != "configured@wso2.com" {
		t.Fatalf("To must be exactly the configured list, got %v", email.to)
	}
	for _, addr := range append(append([]string{}, email.to...), email.cc...) {
		if addr == "am@wso2.com" || addr == "to@wso2.com" {
			t.Fatalf("derived address %q leaked into the recipients", addr)
		}
	}
	if email.subject != Subject {
		t.Errorf("subject: got %q, want %q", email.subject, Subject)
	}
}

func TestSendReport_FetchFailureIsReported(t *testing.T) {
	reports := &fakeReports{err: errors.New("boom")}
	email := &fakeEmail{}
	err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "", true)(context.Background())
	if err == nil {
		t.Fatal("a failed fetch must fail the task so it retries and alerts")
	}
	if email.calls != 0 {
		t.Error("nothing should be sent when the report could not be fetched")
	}
}

func TestSendReport_SendFailureIsReported(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{err: errors.New("smtp down")}
	if err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "", true)(context.Background()); err == nil {
		t.Fatal("a failed send must fail the task")
	}
}

// The rendered body must carry the seven-column shape and the report's own
// figures.
func TestSendReport_RendersTheSevenColumnTable(t *testing.T) {
	reports, email := &fakeReports{report: sampleReport()}, &fakeEmail{}
	if err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "https://wso2.lightning.force.com", true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"Weekly Query Support Consumption Report",
		"Hours Exceeded Accounts",
		"Accounts Going to Exceed",
		"Intrepid Travel",
		"Integration #WSO2CONPI",
		"- Project Key : <b>INTREPIDSUBSUB</b>",
		"Auto Generated by WSO2 Support System",
		"2026-09-20",
		// 6000 minutes entitlement, 6275 consumed, 275 over.
		" 100h 0m",
		" 104h 35m",
		"- 4h 35m",
		// Salesforce links are built from the configured instance.
		"https://wso2.lightning.force.com/lightning/r/Account/acct-sf/view",
		"https://wso2.lightning.force.com/lightning/r/Opportunity/opp-sf/view",
		"https://wso2.lightning.force.com/lightning/r/Project__c/proj-sf/view",
	} {
		if !strings.Contains(email.body, want) {
			t.Errorf("rendered report is missing %q", want)
		}
	}
}

// With no Salesforce instance configured the cells stay plain text rather
// than rendering a broken link.
func TestSendReport_WithoutSalesforceURLCellsArePlainText(t *testing.T) {
	reports, email := &fakeReports{report: sampleReport()}, &fakeEmail{}
	if err := SendReport(reports, email, []string{"a@wso2.com"}, nil, "", true)(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(email.body, "/lightning/r/") {
		t.Error("no Salesforce base URL was configured, so no Salesforce links should be rendered")
	}
	if !strings.Contains(email.body, "Intrepid Travel") {
		t.Error("the account name must still appear as plain text")
	}
}
