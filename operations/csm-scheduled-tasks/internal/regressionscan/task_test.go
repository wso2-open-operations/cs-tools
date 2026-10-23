package regressionscan_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionscan"
)

type fakeReports struct {
	report      regressionreport.Report
	err         error
	calls       int
	gotProducts []string
}

func (f *fakeReports) Report(_ context.Context, products []string) (regressionreport.Report, error) {
	f.calls++
	f.gotProducts = products
	return f.report, f.err
}

type fakeEmail struct {
	to, cc  []string
	subject string
	body    string
	calls   int
	err     error
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc []string, subject, htmlBody string) error {
	f.calls++
	f.to, f.cc, f.subject, f.body = to, cc, subject, htmlBody
	return f.err
}

func iam() regressionscan.Variant {
	return regressionscan.Variant{Key: "iam", Label: "IAM", Products: []string{"WSO2 Identity Server"}}
}

func sampleReport() regressionreport.Report {
	loc, _ := time.LoadLocation("Asia/Colombo")
	return regressionreport.Report{
		GeneratedAt: time.Date(2026, 10, 1, 3, 30, 0, 0, time.UTC),
		Month:       "2026-09",
		PeriodStart: time.Date(2026, 9, 1, 0, 0, 0, 0, loc),
		PeriodEnd:   time.Date(2026, 10, 1, 0, 0, 0, 0, loc),
		Tag:         "s_rg",
		Products:    []string{"WSO2 Identity Server"},
		TotalCases:  2,
		Groups: []regressionreport.Group{{
			Product: "WSO2 Identity Server",
			Cases: []regressionreport.Row{
				{ID: "11111111-1111-1111-1111-111111111111", Number: "CS0000001", IssueKey: "ACME-1",
					Subject: "[IS 7.1.0] Error while executing migration client", Product: "WSO2 Identity Server",
					DeployedProduct: "Identity Server 7.1.0 All In One", EtaSharedOn: time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)},
				{ID: "22222222-2222-2222-2222-222222222222", Number: "CS0000002", IssueKey: "ACME-2",
					Subject: "removing <tenant> from UI", Product: "WSO2 Identity Server",
					EtaSharedOn: time.Date(2026, 9, 20, 4, 0, 0, 0, time.UTC)},
			},
		}},
	}
}

func TestSendReportMailsTheConfiguredRecipients(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := regressionscan.SendReport(reports, email, iam(),
		[]string{"iam-cs@example.com"}, []string{"leadership@example.com"},
		"https://support.example.com/cases/", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if strings.Join(reports.gotProducts, ",") != "WSO2 Identity Server" {
		t.Errorf("products asked for = %v", reports.gotProducts)
	}
	if email.calls != 1 {
		t.Fatalf("SendEmail calls = %d, want 1", email.calls)
	}
	if strings.Join(email.to, ",") != "iam-cs@example.com" || strings.Join(email.cc, ",") != "leadership@example.com" {
		t.Errorf("to = %v, cc = %v", email.to, email.cc)
	}
	if email.subject != "Monthly Product Regression Report - IAM" {
		t.Errorf("subject = %q", email.subject)
	}
	for _, want := range []string{
		"regressions identified in IAM products during the past month",
		"(September 2026)",
		"2 Cases",
		"WSO2 Product: WSO2 Identity Server <span", "(2)</span>",
		`<a href="https://support.example.com/cases/CS0000001" target="_blank" style="color:#ff7300;">ACME-1</a>`,
		"[IS 7.1.0] Error while executing migration client",
		"Identity Server 7.1.0 All In One",
		"removing &lt;tenant&gt; from UI",
		"Perform an RCA for each regression issue",
		"CS Team",
	} {
		if !strings.Contains(email.body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(email.body, "<tenant>") {
		t.Errorf("body carries an unescaped subject")
	}
	if strings.Contains(email.body, "<!-- [") {
		t.Errorf("body still carries a template placeholder")
	}
}

func TestSendReportOmitsAnEmptyMonth(t *testing.T) {
	report := sampleReport()
	report.TotalCases, report.Groups = 0, nil
	email := &fakeEmail{}
	handler := regressionscan.SendReport(&fakeReports{report: report}, email, iam(), []string{"iam-cs@example.com"}, nil, "", true)

	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if email.calls != 0 {
		t.Errorf("an empty month must not be mailed; calls = %d", email.calls)
	}
}

func TestSendReportSkipsWithoutRecipientsOrWhenDisabled(t *testing.T) {
	reports := &fakeReports{report: sampleReport()}
	email := &fakeEmail{}
	handler := regressionscan.SendReport(reports, email, iam(), nil, []string{"cc-only@example.com"}, "", true)
	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	handler = regressionscan.SendReport(reports, email, iam(), []string{"a@example.com"}, nil, "", false)
	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if reports.calls != 0 || email.calls != 0 {
		t.Errorf("calls = reports %d, email %d; want none without a \"to\" or with emails disabled", reports.calls, email.calls)
	}
}

func TestSendReportWrapsFetchAndSendErrors(t *testing.T) {
	boom := errors.New("boom")
	handler := regressionscan.SendReport(&fakeReports{err: boom}, &fakeEmail{}, iam(), []string{"a@example.com"}, nil, "", true)
	if err := handler(context.Background()); !errors.Is(err, boom) {
		t.Errorf("fetch error = %v, want wrapped %v", err, boom)
	}
	email := &fakeEmail{err: boom}
	handler = regressionscan.SendReport(&fakeReports{report: sampleReport()}, email, iam(), []string{"a@example.com"}, nil, "", true)
	if err := handler(context.Background()); !errors.Is(err, boom) {
		t.Errorf("send error = %v, want wrapped %v", err, boom)
	}
}

func TestRenderedBodyWithoutLinkBaseIsPlainText(t *testing.T) {
	email := &fakeEmail{}
	handler := regressionscan.SendReport(&fakeReports{report: sampleReport()}, email, iam(), []string{"a@example.com"}, nil, "", true)
	if err := handler(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if strings.Contains(email.body, "<a href") {
		t.Errorf("no link base configured, but the body carries a link")
	}
	if !strings.Contains(email.body, ">ACME-1<") {
		t.Errorf("body lacks the plain case id")
	}
}

func TestDefaultVariantsAndNames(t *testing.T) {
	variants := regressionscan.DefaultVariants()
	if len(variants) != 3 {
		t.Fatalf("variants = %d, want 3", len(variants))
	}
	names := map[string]bool{}
	for _, v := range variants {
		names[regressionscan.TaskName(v)] = true
		if len(v.Products) == 0 {
			t.Errorf("%s has no products", v.Key)
		}
	}
	for _, want := range []string{"product_regression_report_apim", "product_regression_report_iam", "product_regression_report_integration"} {
		if !names[want] {
			t.Errorf("missing task %s", want)
		}
	}
	if got := regressionscan.Subject(variants[0]); got != "Monthly Product Regression Report - APIM" {
		t.Errorf("subject = %q", got)
	}
}

func TestApplyProductOverrides(t *testing.T) {
	base := regressionscan.DefaultVariants()

	same, err := regressionscan.ApplyProductOverrides(base, "  ")
	if err != nil || len(same) != 3 {
		t.Fatalf("blank overrides: %v, %d variants", err, len(same))
	}

	got, err := regressionscan.ApplyProductOverrides(base, `{"IAM": [" WSO2 Identity Server ", "", "Asgardeo"]}`)
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	for _, v := range got {
		if v.Key == "iam" && strings.Join(v.Products, "|") != "WSO2 Identity Server|Asgardeo" {
			t.Errorf("iam products = %v, want trimmed with blanks dropped, key matched ignoring case", v.Products)
		}
		if v.Key == "apim" && len(v.Products) != len(base[0].Products) {
			t.Errorf("apim products changed by an iam override")
		}
	}
	if base[1].Products[0] != "WSO2 Identity Server" || len(base[1].Products) != 1 {
		t.Errorf("the defaults were mutated: %v", base[1].Products)
	}

	for _, bad := range []string{`{"analytics": ["x"]}`, `{"iam": []}`, `{"iam": [" "]}`, `["iam"]`, `{`} {
		if _, err := regressionscan.ApplyProductOverrides(base, bad); err == nil {
			t.Errorf("%s: want an error", bad)
		}
	}
}
