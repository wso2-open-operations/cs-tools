// Package regressionscan is the monthly product regression report
// sub-cron family: one task per business unit, each fetching last month's
// regression cases for that unit's products from entity-service and
// mailing them as one table, grouped by product. Three units are built in
// (API management, identity, integration), each with the product list its
// report has always covered; the lists can be overridden without a code
// change. Built the way this component's own CLAUDE.md ("Per-task report
// emails") describes: its own template, its own render function, and its
// recipients wired through cmd/server/main.go.
package regressionscan

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
)

// ReportFetcher is the subset of *regressionreport.Client this package
// depends on.
type ReportFetcher interface {
	Report(ctx context.Context, products []string) (regressionreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Variant is one business unit's report: the unit's label, as it appears
// in the mail subject, and the products the report covers.
type Variant struct {
	// Key names the variant in config and in the task name; lower-case,
	// stable, never shown to a reader.
	Key string
	// Label is the unit as the subject and body name it.
	Label string
	// Products are the product names the report covers, matched by
	// entity-service ignoring case.
	Products []string
}

// DefaultVariants are the three reports and the product lists each has
// always covered. A product that no longer exists under that name simply
// contributes no rows.
func DefaultVariants() []Variant {
	return []Variant{
		{Key: "apim", Label: "APIM", Products: []string{
			"WSO2 API Manager",
			"WSO2 Microgateway",
		}},
		{Key: "iam", Label: "IAM", Products: []string{
			"WSO2 Identity Server",
		}},
		{Key: "integration", Label: "Integration", Products: []string{
			"WSO2 Enterprise Integrator",
			"WSO2 Enterprise Service Bus",
			"WSO2 Integrator: BI",
			"WSO2 Integrator: ICP",
			"WSO2 Integrator: MI",
			"WSO2 Integrator: SI",
			"WSO2 MI Monitoring Dashboard",
		}},
	}
}

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS)
// name of a variant's task: product_regression_report_<key>.
func TaskName(v Variant) string {
	return "product_regression_report_" + v.Key
}

// Subject is the variant's mail subject, the subject the report has always
// had.
func Subject(v Variant) string {
	return "Monthly Product Regression Report - " + v.Label
}

// ApplyProductOverrides replaces variants' product lists from raw, a JSON
// object of variant key to product names: {"apim": ["WSO2 API Manager"]}.
// Keys are matched ignoring case. Empty raw leaves every list as it is. A
// key that names no variant, or an empty list, is an error: both would
// silently produce an empty report every month.
func ApplyProductOverrides(variants []Variant, raw string) ([]Variant, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return variants, nil
	}
	var overrides map[string][]string
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		return nil, fmt.Errorf("product overrides are not a JSON object of key to product names: %w", err)
	}
	out := make([]Variant, len(variants))
	copy(out, variants)
	for key, products := range overrides {
		cleaned := make([]string, 0, len(products))
		for _, p := range products {
			if p = strings.TrimSpace(p); p != "" {
				cleaned = append(cleaned, p)
			}
		}
		if len(cleaned) == 0 {
			return nil, fmt.Errorf("product overrides: %q lists no products", key)
		}
		found := false
		for i := range out {
			if strings.EqualFold(out[i].Key, key) {
				out[i].Products = cleaned
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("product overrides: %q is not a report; the reports are %s", key, keys(out))
		}
	}
	return out, nil
}

// keys names the variants, comma-separated, for an error message.
func keys(variants []Variant) string {
	names := make([]string, 0, len(variants))
	for _, v := range variants {
		names = append(names, v.Key)
	}
	return strings.Join(names, ", ")
}

// SendReport returns a registry.Task.Handler that fetches last month's
// report for the variant's products and mails it to to/cc.
//
// The audience is this task's own SUB_CRON_RECIPIENTS entry, nothing else:
// the readers are business-unit mail groups, configured, as they always
// were. When the task has no "to", the handler succeeds without contacting
// entity-service, the same as the stale and open case reports: no report
// to send means no reason to run the query. When the month has no
// regression cases the report is not sent at all -- the source report was
// set to omit itself when it had no records, and an empty table every
// month would train readers to ignore the mail.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED. When false the
// handler succeeds without contacting entity-service.
func SendReport(reports ReportFetcher, email EmailSender, v Variant, to, cc []string, caseLinkBase string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}
		if len(to) == 0 {
			slog.Info("product regression report has no recipients; skipping", "report", v.Label)
			return nil
		}

		report, err := reports.Report(ctx, v.Products)
		if err != nil {
			return fmt.Errorf("regressionscan %s: fetch report: %w", v.Key, err)
		}
		if report.TotalCases == 0 {
			slog.Info("product regression report has no cases; omitted",
				"report", v.Label, "month", report.Month, "products", len(v.Products))
			return nil
		}
		slog.Info("sending the product regression report",
			"report", v.Label, "month", report.Month,
			"cases", report.TotalCases, "products", len(report.Groups),
			"to", len(to), "cc", len(cc))

		body := notify.RenderProductRegressionReport(report, v.Label, caseLinkBase)
		if err := email.SendEmail(ctx, to, cc, Subject(v), body); err != nil {
			return fmt.Errorf("regressionscan %s: send report: %w", v.Key, err)
		}
		return nil
	}
}
