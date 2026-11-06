// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package aws

import (
	"context"
	"regexp"
	"strings"
	"time"

	"sre-alert-ingestion-service/utils"
)

// budgetSubjectPrefix starts the Subject of every AWS Budgets notification.
const budgetSubjectPrefix = "AWS Budgets:"

// budgetTagTimeout caps the budget tag lookup per alert so a slow AWS call never stalls ingestion.
const budgetTagTimeout = 5 * time.Second

var (
	budgetNameLine = regexp.MustCompile(`(?m)^\s*Budget Name:\s*(.+?)\s*$`)
	budgetAccount  = regexp.MustCompile(`AWS Account:?\s*(\d{12})`)
	// budgetSubjectName is the budget named in "AWS Budgets: <name> has exceeded ..." or "... is forecasted to exceed ...".
	budgetSubjectName = regexp.MustCompile(`^AWS Budgets:\s*(.+?)\s+(?:has exceeded|is forecasted to exceed|has|is)\b`)
)

// isBudget reports whether a plain-text SNS notification is an AWS Budgets alert.
func isBudget(subject, message string) bool {
	return strings.HasPrefix(subject, budgetSubjectPrefix) || budgetNameLine.MatchString(message)
}

// fromBudget maps an AWS Budgets alert: one incident per budget and account, whichever threshold or forecast fired.
// A budget alert defaults to critical; the budget's own tags stand in for the AlarmDescription overrides budgets lack.
func fromBudget(subject, message string, base Alert, tagger BudgetTagFetcher) Alert {
	name := ""
	if m := budgetNameLine.FindStringSubmatch(message); m != nil {
		name = m[1]
	} else if m := budgetSubjectName.FindStringSubmatch(subject); m != nil {
		name = m[1]
	}
	account := ""
	if m := budgetAccount.FindStringSubmatch(message); m != nil {
		account = m[1]
	}

	base.MetricName = utils.FirstNonEmpty(subject, "AWS Budgets alert")
	if name != "" {
		base.MetricName = budgetSubjectPrefix + " " + name
		base.UniqueIdentifier = strings.Trim(account+"/"+name, "/")
	}
	base.SourceAccount = utils.FirstNonEmpty(account, base.SourceAccount)
	base.Description = budgetSummary(message)
	base.Severity = "critical"
	base.Category = "service_interruption"

	if tagger != nil && name != "" && base.SourceAccount != "" {
		ctx, cancel := context.WithTimeout(context.Background(), budgetTagTimeout)
		defer cancel()
		// A failed lookup (no credentials, another account's budget, no such budget) keeps the configured values.
		if tags, err := tagger.Tags(ctx, base.SourceAccount, name); err == nil {
			base.Service = utils.FirstNonEmpty(tagValue(tags, "service"), base.Service)
			base.Environment = utils.FirstNonEmpty(tagValue(tags, "environment"), base.Environment)
			base.Severity = utils.FirstNonEmpty(tagValue(tags, "severity"), base.Severity)
			base.Category = utils.FirstNonEmpty(tagValue(tags, "category"), base.Category)
			base.AssignmentGroup = tagValue(tags, "assignment_group")
		}
	}
	return base
}

// tagValue reads a budget tag by key: the exact key wins, else the lexically first case-insensitive match ("Service").
func tagValue(tags map[string]string, key string) string {
	if v, ok := tags[key]; ok {
		return strings.TrimSpace(v)
	}
	match := ""
	for k := range tags {
		if strings.EqualFold(k, key) && (match == "" || k < match) {
			match = k
		}
	}
	return strings.TrimSpace(tags[match])
}

// budgetSummary is the "You requested that we alert you ..." sentence, or the whole message when it has none.
func budgetSummary(message string) string {
	for _, line := range strings.Split(message, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "You requested that we alert you") {
			return line
		}
	}
	return strings.TrimSpace(message)
}
