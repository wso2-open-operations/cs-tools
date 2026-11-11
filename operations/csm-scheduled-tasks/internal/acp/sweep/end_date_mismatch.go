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

package sweep

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/closure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
)

// A project's end date is decided from Salesforce's (see salesforceDetails),
// but staff sometimes extend it by hand in CSM only. Acting on Salesforce's
// earlier date would then notify and suspend a customer whose term was
// renewed. So when CSM's date is later than Salesforce's and Salesforce's
// date would trigger a notice or a suspension, the end-date track does
// nothing and the Account Manager is asked, at most once a week, to correct
// Salesforce. Nothing is persisted: the weekly cadence comes from sending
// only on the full-sweep weekday.

// csmEndDateLater reports whether the end date CSM held before Salesforce's
// replaced it is a later calendar day (UTC) than Salesforce's. Both must be
// known.
func csmEndDateLater(proj project) bool {
	if proj.csmEndDate == nil || proj.EndDate == nil {
		return false
	}
	return utcDay(*proj.csmEndDate).After(utcDay(*proj.EndDate))
}

func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// endDateMismatchSubject is the warning's title line, [ACP]-prefixed like the
// internal notices.
func endDateMismatchSubject(projectName, accountName string) string {
	return fmt.Sprintf("[ACP] End Date Mismatch for Project %s", projectOfAccount(projectName, accountName))
}

const endDateMismatchBodyTemplate = `Dear %[1]s

The end date of the following project is later in CSM than in Salesforce.

Project Name: %[2]s

Project Key: %[3]s

CSM End Date: %[4]s

Salesforce End Date: %[5]s

No notice or suspension was sent for this project because of the mismatch. Please correct the end date in Salesforce so that it matches CSM. If the dates still differ, you may receive this message again next week.

Best Regards,
WSO2 Team`

func endDateMismatchBody(proj project, accountManagerName string) string {
	if accountManagerName == "" {
		accountManagerName = "Account Manager"
	}
	return fmt.Sprintf(endDateMismatchBodyTemplate, accountManagerName, proj.Name, proj.ProjectKey,
		formatDate(proj.csmEndDate), formatDate(proj.EndDate))
}

// warnEndDateMismatch is called for a project whose end-date track would act
// on Salesforce's date (decision) while CSM's date is later. The caller drops
// the track either way; this only sends the weekly warning, and only on the
// warning weekday. It never fails the project: a warning that could not be
// sent is logged and tried again next week.
func warnEndDateMismatch(ctx context.Context, reader entityReader, ntf notifier, proj project, decision closure.Decision) {
	attrs := []any{
		"projectID", proj.ID, "projectKey", proj.ProjectKey,
		"csmEndDate", formatDate(proj.csmEndDate), "salesforceEndDate", formatDate(proj.EndDate),
	}
	if !proj.endDateWarnDay {
		slog.InfoContext(ctx, "acp: end-date track skipped, CSM end date is later than Salesforce's; warning is sent on the weekly sweep day only",
			append(attrs, "warningSent", false)...)
		return
	}

	contacts, err := resolveAccountContacts(ctx, reader, proj.accountID())
	if err != nil {
		slog.WarnContext(ctx, "acp: end-date track skipped, CSM end date is later than Salesforce's; warning not sent: account could not be read",
			append(attrs, "warningSent", false, "err", err)...)
		return
	}

	notice := baseNotice(proj, decision.Window)
	notice.EndDate = timeValue(proj.EndDate)
	notice.Subject = endDateMismatchSubject(proj.Name, accountName(proj))
	notice.Body = endDateMismatchBody(proj, contacts.AccountOwner.Name)
	notice.Recipients = notify.Recipients{AccountOwner: recipients.Contact{Name: contacts.AccountOwner.Name, Email: contacts.AccountOwner.Email}}
	notice.StandingAsCC = true

	delivered, err := ntf.Send(ctx, notice)
	if err != nil {
		slog.WarnContext(ctx, "acp: end-date track skipped, CSM end date is later than Salesforce's; warning failed to send",
			append(attrs, "warningSent", false, "err", err)...)
		return
	}
	slog.WarnContext(ctx, "acp: end-date track skipped, CSM end date is later than Salesforce's",
		append(attrs, "warningSent", delivered)...)
}
