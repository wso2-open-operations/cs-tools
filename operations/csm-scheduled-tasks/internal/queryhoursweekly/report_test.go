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
	"reflect"
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

// The To line is the seed plus both owners of every EXCEEDED account, which
// is what the ServiceNow action script builds. Cc stays configuration.
func TestSendReport_AddressesAreDerivedFromTheData(t *testing.T) {
	report := sampleReport()
	report.Exceeded[0].AccountManagerEmail = "am@wso2.com"
	report.Exceeded[0].TechnicalOwnerEmail = "to@wso2.com"

	reports, email := &fakeReports{report: report}, &fakeEmail{}
	err := SendReport(reports, email, []string{"seed@wso2.com"}, []string{"cc@wso2.com"}, "", true)(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"seed@wso2.com", "am@wso2.com", "to@wso2.com"}
	if !reflect.DeepEqual(email.to, want) {
		t.Fatalf("To: got %v, want %v", email.to, want)
	}
	// Cc is NOT derived -- the script never pushes onto it.
	if !reflect.DeepEqual(email.cc, []string{"cc@wso2.com"}) {
		t.Fatalf("Cc must stay configuration, got %v", email.cc)
	}
	if email.subject != Subject {
		t.Errorf("subject: got %q, want %q", email.subject, Subject)
	}
}

// *** THE REGRESSION THIS GUARDS IS THE EXPENSIVE ONE. *** In the action
// script both pushes sit inside `if (acc_data.is_exceeded)`; the
// close-to-exceed branch appends HTML and nobody. Deriving from both tables
// reads as a tidy symmetry and silently widens the weekly audience.
func TestDeriveRecipients_GoingToExceedContributesNobody(t *testing.T) {
	report := queryhoursreport.Report{
		Exceeded: []queryhoursreport.Account{{
			AccountManagerEmail: "exceeded-am@wso2.com",
		}},
		GoingToExceed: []queryhoursreport.Account{{
			AccountManagerEmail: "close-am@wso2.com",
			TechnicalOwnerEmail: "close-to@wso2.com",
		}},
	}

	got := DeriveRecipients(report, []string{"seed@wso2.com"})
	want := []string{"seed@wso2.com", "exceeded-am@wso2.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeriveRecipients(t *testing.T) {
	tests := []struct {
		name string
		seed []string
		accs []queryhoursreport.Account
		want []string
	}{
		{
			name: "seed alone when nothing is exceeded",
			seed: []string{"seed@wso2.com"},
			want: []string{"seed@wso2.com"},
		},
		{
			name: "one account can contribute two addresses",
			seed: []string{"seed@wso2.com"},
			accs: []queryhoursreport.Account{{
				AccountManagerEmail: "am@wso2.com",
				TechnicalOwnerEmail: "tech@wso2.com",
			}},
			want: []string{"seed@wso2.com", "am@wso2.com", "tech@wso2.com"},
		},
		{
			// `arrayUtil.unique` after `toLowerCase`: without the fold these
			// are two recipients and the same person is mailed twice.
			name: "duplicates are folded case-insensitively",
			seed: []string{"seed@wso2.com"},
			accs: []queryhoursreport.Account{
				{AccountManagerEmail: "Shared@WSO2.com"},
				{AccountManagerEmail: "shared@wso2.com", TechnicalOwnerEmail: "SHARED@wso2.com"},
			},
			want: []string{"seed@wso2.com", "shared@wso2.com"},
		},
		{
			name: "non-wso2 addresses are dropped",
			seed: []string{"seed@wso2.com"},
			accs: []queryhoursreport.Account{{
				AccountManagerEmail: "customer@example.com",
				TechnicalOwnerEmail: "tech@wso2.com",
			}},
			want: []string{"seed@wso2.com", "tech@wso2.com"},
		},
		{
			// An owner reference whose user row has no email arrives empty;
			// ServiceNow drops it the same way, via the domain test.
			name: "blank owner emails are skipped",
			seed: []string{"seed@wso2.com"},
			accs: []queryhoursreport.Account{{
				AccountManagerEmail: "",
				TechnicalOwnerEmail: "   ",
			}},
			want: []string{"seed@wso2.com"},
		},
		{
			name: "seed is deduped against a derived address and keeps the head",
			seed: []string{"seed@wso2.com"},
			accs: []queryhoursreport.Account{{AccountManagerEmail: "seed@wso2.com"}},
			want: []string{"seed@wso2.com"},
		},
		{
			// The seed is operator-configured, so it is exempt from the
			// domain test; derived addresses never are.
			name: "a non-wso2 seed is kept",
			seed: []string{"ops@partner.example"},
			accs: []queryhoursreport.Account{{AccountManagerEmail: "am@wso2.com"}},
			want: []string{"ops@partner.example", "am@wso2.com"},
		},
		{
			name: "no seed derives from the data alone",
			accs: []queryhoursreport.Account{{AccountManagerEmail: "am@wso2.com"}},
			want: []string{"am@wso2.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveRecipients(queryhoursreport.Report{Exceeded: tc.accs}, tc.seed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
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
