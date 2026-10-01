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
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

// liveTestRecipient is the ONLY address a live run may deliver to.
//
// *** THE WHOLE POINT OF THIS FILE IS THAT A REAL RUN CANNOT MAIL ANYBODY
// ELSE. *** This task's To line is DERIVED from live data, so a faithful
// end-to-end run would address every exceeded account's account manager and
// technical owner — real people, roughly fifty of them. The capping sender
// below is the single choke point that prevents that, and the guard in the
// test refuses to start if this constant is ever widened.
const liveTestRecipient = "sasmitha@wso2.com"

// cappingSender runs the real email client but REPLACES the recipients.
//
// SendReport hands it the genuinely derived To line, which is what makes this
// a real test of the derivation — and then this type throws that list away and
// delivers to liveTestRecipient alone, Cc dropped entirely because Cc is
// configuration and would be real colleagues too.
//
// The derived list is LOGGED rather than used, which is the other reason to
// run this: it is the only way to see the real recipient count without mailing
// it. The dev ServiceNow copy capped near 33 against the real message's ~48,
// and that gap is still unexplained.
type cappingSender struct {
	inner   *notify.Client
	t       *testing.T
	derived []string
}

func (c *cappingSender) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	c.derived = append([]string{}, to...)

	c.t.Logf("")
	c.t.Logf("=== DERIVED To LINE — %d address(es), NOT delivered to ===", len(to))
	for i, address := range to {
		c.t.Logf("  %2d. %s", i+1, address)
	}
	if len(cc) > 0 {
		c.t.Logf("=== configured Cc — %d address(es), DROPPED ===", len(cc))
		for i, address := range cc {
			c.t.Logf("  %2d. %s", i+1, address)
		}
	}
	c.t.Logf("=== delivering to %s only ===", liveTestRecipient)
	c.t.Logf("")

	return c.inner.SendEmail(ctx, []string{liveTestRecipient}, nil, subject, htmlBody)
}

// TestLiveSendWeeklyReport fetches the real weekly report from a running
// entity-service, runs the real SendReport over it, and delivers the rendered
// mail to liveTestRecipient only.
//
// Skipped unless QH_LIVE_REPORT_BASE_URL is set, so an ordinary `go test`
// never sends anything. Point it at the harness in
// entity-service/internal/server/query_hours_report_harness_test.go.
//
// Everything on the real path runs: the OAuth2 client-credentials fetch, the
// HTTP call, the JSON decode into the wire types, DeriveRecipients, the
// html/template render, and the email service call. Only the recipient list is
// intercepted, and only at the final hop.
func TestLiveSendWeeklyReport(t *testing.T) {
	base := os.Getenv("QH_LIVE_REPORT_BASE_URL")
	if base == "" {
		t.Skip("QH_LIVE_REPORT_BASE_URL not set; this run sends real email")
	}

	// Refuse to run if the constant has been edited to widen delivery. A
	// comma would smuggle a second address through a single string.
	if liveTestRecipient != "sasmitha@wso2.com" || strings.Contains(liveTestRecipient, ",") {
		t.Fatalf("refusing to run: recipient is %q, expected exactly sasmitha@wso2.com", liveTestRecipient)
	}

	reportClient, err := queryhoursreport.NewClient(queryhoursreport.Config{
		BaseURL:      base,
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
	})
	if err != nil {
		t.Fatalf("report client: %v", err)
	}

	emailClient, err := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		t.Fatalf("email client: %v", err)
	}

	sender := &cappingSender{inner: emailClient, t: t}

	// The seed is this one address, so a run where nothing is exceeded still
	// has a To line and still sends — which is itself worth seeing.
	seed := []string{liveTestRecipient}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := SendReport(reportClient, sender, seed, nil,
		os.Getenv("SALESFORCE_BASE_URL"), true)(ctx); err != nil {
		t.Fatalf("SendReport: %v", err)
	}

	if len(sender.derived) == 0 {
		t.Fatal("SendReport did not reach the sender; nothing was delivered")
	}
	t.Logf("delivered to %s; derived list was %d address(es)", liveTestRecipient, len(sender.derived))
}

// fixtureReport is a report shaped like a real one but built from synthetic
// accounts, so the derivation has something to derive from.
//
// *** WHY THIS EXISTS. *** The dev database has ZERO approved time cards, and
// project_consumed counts only state='APPROVED'. Consumption is therefore 0
// for every project, remaining can never go negative, and nothing is ever
// exceeded — so a live run against dev exercises the whole pipeline and
// derives an empty audience, proving nothing about DeriveRecipients. This
// fixture covers that gap without writing to a shared database.
//
// The addresses are deliberately non-existent wso2.com mailboxes: they must
// pass the domain filter to exercise it, and they are never delivered to
// because cappingSender replaces the recipient list.
func fixtureReport() queryhoursreport.Report {
	project := func(name, key string, consumed int) queryhoursreport.Project {
		return queryhoursreport.Project{Name: name, Key: key, SFID: "a0p" + key, ConsumedMinutes: consumed}
	}
	group := func(entitlement, consumed int, opps ...queryhoursreport.Opportunity) queryhoursreport.Group {
		remaining := entitlement - consumed
		return queryhoursreport.Group{
			Opportunities:      opps,
			EntitlementMinutes: entitlement,
			ConsumedMinutes:    consumed,
			RemainingMinutes:   remaining,
			Exceeded:           remaining < 0,
			GoingToExceed:      remaining >= 0 && remaining < 600,
			RowCount:           len(opps),
		}
	}

	return queryhoursreport.Report{
		GeneratedOn:        time.Now().UTC().Format("2006-01-02"),
		ExceededCount:      2,
		GoingToExceedCount: 1,
		Exceeded: []queryhoursreport.Account{
			{
				Name: "Northwind Logistics", SFID: "a0a000000000000001", Exceeded: true, RowCount: 1,
				AccountManagerEmail: "qh-live-am-northwind@wso2.com",
				TechnicalOwnerEmail: "qh-live-to-northwind@wso2.com",
				Groups: []queryhoursreport.Group{group(6000, 7320,
					queryhoursreport.Opportunity{
						Name: "Integration Platform Renewal FY26", SFID: "a0o000000000000001",
						EntitlementMinutes: 6000,
						Projects:           []queryhoursreport.Project{project("Northwind - API Manager", "NWAPIM", 7320)},
					})},
			},
			{
				// Same technical owner as the account above, and an account
				// manager whose address differs only by case — both must fold
				// to one recipient each.
				Name: "Pacific Health Group", SFID: "a0a000000000000002", Exceeded: true, RowCount: 2,
				AccountManagerEmail: "QH-Live-AM-Northwind@WSO2.com",
				TechnicalOwnerEmail: "qh-live-to-northwind@wso2.com",
				Groups: []queryhoursreport.Group{group(3000, 3180,
					queryhoursreport.Opportunity{
						Name: "Choreo Subscription FY26", SFID: "a0o000000000000002",
						EntitlementMinutes: 3000,
						Projects: []queryhoursreport.Project{
							project("Pacific - Choreo CP", "PACCP", 2400),
							project("Pacific - Choreo DP", "PACDP", 780),
						},
					})},
			},
		},
		GoingToExceed: []queryhoursreport.Account{
			{
				// *** CONTRIBUTES NOBODY. *** If either address below shows up
				// in the derived list, the exceeded-only rule has regressed.
				Name: "Cascade Retail", SFID: "a0a000000000000003", GoingToExceed: true, RowCount: 1,
				AccountManagerEmail: "qh-live-am-cascade@wso2.com",
				TechnicalOwnerEmail: "qh-live-to-cascade@wso2.com",
				Groups: []queryhoursreport.Group{group(6000, 5700,
					queryhoursreport.Opportunity{
						Name: "Asgardeo Expansion FY26", SFID: "a0o000000000000003",
						EntitlementMinutes: 6000,
						Projects:           []queryhoursreport.Project{project("Cascade - Asgardeo", "CASIAM", 5700)},
					})},
			},
		},
	}
}

type fixtureFetcher struct{ report queryhoursreport.Report }

func (f fixtureFetcher) WeeklyReport(context.Context) (queryhoursreport.Report, error) {
	return f.report, nil
}

// TestLiveSendWeeklyReportFixture sends a real email rendered from
// fixtureReport, exercising DeriveRecipients against a populated audience and
// the template against a filled two-table report.
//
// Skipped unless QH_LIVE_FIXTURE_SEND is set. Delivery is capped to
// liveTestRecipient by the same sender as the real-data run.
func TestLiveSendWeeklyReportFixture(t *testing.T) {
	if os.Getenv("QH_LIVE_FIXTURE_SEND") == "" {
		t.Skip("QH_LIVE_FIXTURE_SEND not set; this run sends real email")
	}
	if liveTestRecipient != "sasmitha@wso2.com" || strings.Contains(liveTestRecipient, ",") {
		t.Fatalf("refusing to run: recipient is %q, expected exactly sasmitha@wso2.com", liveTestRecipient)
	}

	emailClient, err := notify.NewClient(notify.Config{
		BaseURL:      os.Getenv("EMAIL_BASE_URL"),
		TokenURL:     os.Getenv("OAUTH2_TOKEN_URL"),
		ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		t.Fatalf("email client: %v", err)
	}

	sender := &cappingSender{inner: emailClient, t: t}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := SendReport(fixtureFetcher{fixtureReport()}, sender,
		[]string{liveTestRecipient}, []string{"qh-live-cc@wso2.com"},
		os.Getenv("SALESFORCE_BASE_URL"), true)(ctx); err != nil {
		t.Fatalf("SendReport: %v", err)
	}

	// The audience the real task WOULD have addressed: the seed, Northwind's
	// two owners, and nothing from Pacific (duplicate owner, case-folded
	// manager) or Cascade (going to exceed only).
	want := []string{
		liveTestRecipient,
		"qh-live-am-northwind@wso2.com",
		"qh-live-to-northwind@wso2.com",
	}
	if !reflect.DeepEqual(sender.derived, want) {
		t.Fatalf("derived audience:\n got %v\nwant %v", sender.derived, want)
	}
	for _, leaked := range []string{"qh-live-am-cascade@wso2.com", "qh-live-to-cascade@wso2.com"} {
		for _, got := range sender.derived {
			if got == leaked {
				t.Fatalf("going-to-exceed address %q leaked into the audience", leaked)
			}
		}
	}
}
