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

package worklogsreport

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

// MaxWindowDays is the widest inclusive range the entity-service report
// endpoint accepts. A larger request is refused there with a 400; refusing it
// up front gives the operator the reason before anything is fetched.
const MaxWindowDays = 92

// Override is what a manual run-now changes about one report run. The zero
// value changes nothing, and a handler built with it behaves exactly as the
// scheduled one does.
type Override struct {
	// From and To are the inclusive reporting window, plain calendar dates in
	// ReportTimezone held as UTC midnights, like WeeklyRange's. Both zero means
	// the window is derived from the run's anchor as usual.
	From, To time.Time
	// Recipients and Cc REPLACE the audience for this run: the configured lists
	// and the registry-derived audience are both ignored, not merged. Empty
	// Recipients means no replacement, and Cc is then not looked at.
	Recipients, Cc []string
}

// HasWindow reports whether the reporting window is overridden.
func (o Override) HasWindow() bool { return !o.From.IsZero() || !o.To.IsZero() }

// HasRecipients reports whether the audience is replaced.
func (o Override) HasRecipients() bool { return len(o.Recipients) > 0 }

// Active reports whether anything is overridden.
func (o Override) Active() bool { return o.HasWindow() || o.HasRecipients() }

// ValidateWindow checks the window on its own: both ends given, in order, and
// within the span the entity-service accepts.
func ValidateWindow(from, to time.Time) error {
	if from.IsZero() != to.IsZero() {
		return errors.New("--from-date and --to-date must be given together")
	}
	if from.IsZero() {
		return nil
	}
	if to.Before(from) {
		return fmt.Errorf("--to-date %s is before --from-date %s", to.Format(dateLayout), from.Format(dateLayout))
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > MaxWindowDays {
		return fmt.Errorf("the window %s to %s is %d days; the report endpoint accepts at most %d",
			from.Format(dateLayout), to.Format(dateLayout), days, MaxWindowDays)
	}
	return nil
}

// Validate checks the override against the cadence it will run under. The
// weekly report takes any valid window as is; the daily one reports a single
// day, so its window must have from equal to to.
func (o Override) Validate(cadence worklogreport.Cadence) error {
	if err := ValidateWindow(o.From, o.To); err != nil {
		return err
	}
	if o.HasWindow() && cadence != worklogreport.CadenceWeekly && !o.From.Equal(o.To) {
		return fmt.Errorf("the daily report covers a single day: --from-date and --to-date must be equal (got %s and %s)",
			o.From.Format(dateLayout), o.To.Format(dateLayout))
	}
	if len(o.Cc) > 0 && len(o.Recipients) == 0 {
		return errors.New("--cc needs --recipients: it only adds copies to a replaced audience")
	}
	return nil
}

const dateLayout = "2006-01-02"

// ParseDate reads a plain calendar date, YYYY-MM-DD, as a UTC midnight.
func ParseDate(flagName, raw string) (time.Time, error) {
	d, err := time.Parse(dateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s=%q is not a date in the form YYYY-MM-DD", flagName, raw)
	}
	return d, nil
}

// ParseAddressList reads a comma list of bare email addresses. It is strict on
// purpose: a recipient override replaces the audience, and a silently dropped
// or mangled address would send a test to the wrong people or to nobody. An
// empty entry (a trailing comma), any whitespace, a display name, or an address
// without a dotted domain is refused. Duplicates differing only in case are
// collapsed.
func ParseAddressList(flagName, raw string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, entry := range strings.Split(raw, ",") {
		if entry == "" {
			return nil, fmt.Errorf("%s has an empty entry; give a comma list such as jane.doe@example.com,john.roe@example.com", flagName)
		}
		if strings.ContainsAny(entry, " \t\r\n") {
			return nil, fmt.Errorf("%s entry %q contains whitespace; use a plain comma list without spaces", flagName, entry)
		}
		a, err := mail.ParseAddress(entry)
		if err != nil || a.Address != entry || a.Name != "" {
			return nil, fmt.Errorf("%s entry %q is not a plain email address", flagName, entry)
		}
		at := strings.LastIndex(entry, "@")
		if !strings.Contains(entry[at+1:], ".") || strings.HasSuffix(entry, ".") {
			return nil, fmt.Errorf("%s entry %q has no valid domain", flagName, entry)
		}
		if key := strings.ToLower(entry); !seen[key] {
			seen[key] = true
			out = append(out, entry)
		}
	}
	return out, nil
}

// OverrideSubject marks a subject so a manual run cannot be mistaken for the
// scheduled mail. A run with nothing overridden returns the subject untouched.
func OverrideSubject(subject string, o Override) string {
	prefix := ""
	if o.HasRecipients() {
		prefix += "[Test]"
	}
	if o.HasWindow() {
		prefix += "[Manual range]"
	}
	if prefix == "" {
		return subject
	}
	return prefix + " " + subject
}
