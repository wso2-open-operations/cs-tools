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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

// The Account Closure Process's own rules, copied from ACP
// (operations/csm-scheduled-tasks/internal/acp, the acp_closure sub-cron) so
// the reopen check reopens a track exactly when ACP would no longer suspend
// it. Keep the two copies in step: a rule changed in one and not the other
// makes ACP suspend a project every day and this check reopen it, with a
// suspension notice and a "Back to Open state" email each time.
// testdata/acp_rules_parity.json holds the cases both copies must agree on:
// TestACPRulesParity runs them here, and ACP's own parity tests run the same
// file against its rules.
//
//   - days: closure/decide.go daysBetween
//   - invoices: sweep/invoice_resolve.go eligibleOpportunity, eligibleInvoice,
//     excludedInvoiceClassifications, parseEULAVersionDecimal, and
//     sweep/salesforce_details.go eulaVersionDecimal
//   - grace period: closure/decide_invoice.go usesGracePeriod, InvoiceSuspendDate
//   - the suspensionProcessState blob: suspensionstate/suspensionstate.go

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// suspensionProcessState section keys, one per ACP track.
const (
	acpEndDateSection = "based_on_subscription_end_date"
	acpInvoiceSection = "based_on_due_invoices"
)

// acpSuspendEventType is the event_type ACP (and the previous process before
// it) stores in a track's section when it suspends the project on that track.
// It is how the reopen check tells a project ACP suspended from one somebody
// suspended by hand: a manual suspension writes no such record.
const acpSuspendEventType = "suspend"

// acpOpenEventType is the event_type of a track with no notice for its
// current term, which a reopen stores so the reminders start over.
const acpOpenEventType = "open"

// acpSectionEventType returns the event_type of one section of a
// suspensionProcessState blob, "" when the blob, the section or the field is
// absent.
func acpSectionEventType(raw json.RawMessage, key string) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}
	var blob map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blob); err != nil {
		return "", fmt.Errorf("parse suspensionProcessState: %w", err)
	}
	section, ok := blob[key]
	if !ok {
		return "", nil
	}
	var state struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(section, &state); err != nil {
		return "", fmt.Errorf("parse suspensionProcessState %s: %w", key, err)
	}
	return state.EventType, nil
}

// acpWithSectionOpen returns raw with only the given section replaced by
// {"event_type":"open"}; every other key is carried through byte for byte and
// nothing is HTML-escaped (suspensionstate.withSection).
func acpWithSectionOpen(raw json.RawMessage, key string) (json.RawMessage, error) {
	blob := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &blob); err != nil {
			return nil, fmt.Errorf("parse suspensionProcessState: %w", err)
		}
	}
	if blob == nil {
		// A JSON null decodes into a nil map.
		blob = map[string]json.RawMessage{}
	}
	blob[key] = json.RawMessage(`{"event_type":"` + acpOpenEventType + `"}`)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(blob); err != nil {
		return nil, fmt.Errorf("marshal suspensionProcessState: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// acpDaysBetween is the number of whole days from now to end, rounded up when
// end is ahead (closure.daysBetween). ACP suspends at 0 or less, so a track
// may reopen only when this is above 0.
func acpDaysBetween(now, end time.Time) int {
	days := end.Sub(now).Hours() / 24
	if days != float64(int(days)) {
		if days > 0 {
			days = float64(int(days)) + 1
		} else {
			days = float64(int(days))
		}
	}
	return int(days)
}

// acpParseDate reads a Salesforce Date ("2026-10-13", or a datetime whose
// first ten characters are the date) as UTC midnight; nil when blank.
func acpParseDate(v *string) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil, nil
	}
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// acpClosedWonStage is the only opportunity stage whose invoices count.
const acpClosedWonStage = "50 - Closed Won"

// acpCustomerContractEULA is the EULA text whose opportunity's invoices never count.
const acpCustomerContractEULA = "Customer contract"

// acpExcludedInvoiceClassifications are the Cloud support invoice codes
// (PP/CO/TAM) that never count.
var acpExcludedInvoiceClassifications = map[string]bool{"PP": true, "CO": true, "TAM": true}

// acpAutoCreatedPSInvoiceName is the placeholder invoice name that never counts.
const acpAutoCreatedPSInvoiceName = "Auto Created PS"

// acpOpportunityCounts reports whether an opportunity's invoices count: stage
// "50 - Closed Won" and a EULA that is present and is not "Customer contract"
// (sweep eligibleOpportunity). An empty EULA text counts, as in ACP: were it
// skipped here, ACP would suspend on its invoices and this check reopen the
// project every day.
func acpOpportunityCounts(stage, eulaVersion *string) bool {
	return stage != nil && *stage == acpClosedWonStage &&
		eulaVersion != nil && *eulaVersion != acpCustomerContractEULA
}

// acpEULAVersionPattern and acpEULAVersionDecimal give the EULA version as a
// number: the first x.y in the text, 3.4 when the text is empty, 0 (unknown)
// otherwise.
var acpEULAVersionPattern = regexp.MustCompile(`\d+\.\d+`)

func acpEULAVersionDecimal(eulaVersion *string) float64 {
	s := ""
	if eulaVersion != nil {
		s = strings.TrimSpace(*eulaVersion)
	}
	if s == "" {
		return 3.4
	}
	m := acpEULAVersionPattern.FindString(s)
	if m == "" {
		return 0
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0
	}
	return f
}

// acpUsesGracePeriod reports whether an invoice gets the 60-day grace period:
// always with a primary partner, else for a EULA of 3.3 or older.
func acpUsesGracePeriod(eulaVersion float64, hasPrimaryPartner bool) bool {
	return hasPrimaryPartner || (eulaVersion > 0 && eulaVersion <= 3.3)
}

// acpInvoiceSuspendDate is the date ACP suspends an unpaid invoice on: the due
// date, or with the grace period the invoice date plus 60 days unless the due
// date is later still.
func acpInvoiceSuspendDate(invoiceDate, dueDate time.Time, eulaVersion float64, hasPrimaryPartner bool) time.Time {
	if !acpUsesGracePeriod(eulaVersion, hasPrimaryPartner) {
		return dueDate
	}
	if dueDate.Sub(invoiceDate).Hours()/24 > 60 {
		return dueDate
	}
	return invoiceDate.AddDate(0, 0, 60)
}

// acpInvoice is one invoice of a counted opportunity, as the rules read it.
type acpInvoice struct {
	Name           *string
	Classification *string
	InvoiceDate    *time.Time
	DueDate        *time.Time
	PaidDate       *time.Time
	EULAVersion    float64
}

// acpInvoiceCounts reports whether an invoice is one ACP chases: it has a due
// date, is unpaid, is not a Cloud support or "Auto Created PS" invoice, and is
// not due before the project started.
func acpInvoiceCounts(inv acpInvoice, projectStart *time.Time) bool {
	if inv.DueDate == nil || inv.PaidDate != nil {
		return false
	}
	if inv.Classification != nil && acpExcludedInvoiceClassifications[*inv.Classification] {
		return false
	}
	if inv.Name != nil && *inv.Name == acpAutoCreatedPSInvoiceName {
		return false
	}
	if projectStart != nil && inv.DueDate.Before(*projectStart) {
		return false
	}
	return true
}

// acpInvoiceTrackClear reports whether nothing is left on the invoice track
// that ACP would suspend for: no counted invoice, or the earliest due counted
// invoice (the one ACP times the track by) is not past its suspend date. A
// Partner account without a primary partner has no invoice track at all, so
// it is clear too. Invoices with no invoice date count as dated from the zero
// time, as in ACP.
func acpInvoiceTrackClear(now time.Time, invoices []acpInvoice, projectStart *time.Time, isPartner, hasPrimaryPartner bool) bool {
	if isPartner && !hasPrimaryPartner {
		return true
	}
	var counted []acpInvoice
	for _, inv := range invoices {
		if acpInvoiceCounts(inv, projectStart) {
			counted = append(counted, inv)
		}
	}
	if len(counted) == 0 {
		return true
	}
	sort.SliceStable(counted, func(i, j int) bool { return counted[i].DueDate.Before(*counted[j].DueDate) })
	first := counted[0]
	invoiceDate := time.Time{}
	if first.InvoiceDate != nil {
		invoiceDate = *first.InvoiceDate
	}
	suspendOn := acpInvoiceSuspendDate(invoiceDate, *first.DueDate, first.EULAVersion, hasPrimaryPartner)
	return acpDaysBetween(now, suspendOn) > 0
}
