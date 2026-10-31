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
	"encoding/json"
	"testing"
)

const budgetMessage = `AWS Budget Notification October 28, 2026
AWS Account 123456789012

Dear AWS Customer,

You requested that we alert you when the ACTUAL Cost associated with your monthly-cost budget is greater than $80.00 for the current month. The ACTUAL Cost associated with this budget is $85.12. You can find additional details below and by accessing the AWS Budgets dashboard [1].

Budget Name: monthly-cost
Budget Type: Cost
Budgeted Amount: $100.00
Alert Type: ACTUAL
Alert Threshold: > $80.00
ACTUAL Amount: $85.12
`

func budgetEnvelope(subject, message string) []byte {
	raw, _ := json.Marshal(map[string]any{
		"Type": "Notification", "MessageId": "m-1", "TopicArn": "arn:aws:sns:us-east-1:999988887777:budgets",
		"Subject": subject, "Message": message,
	})
	return raw
}

func TestTransform_BudgetAlert(t *testing.T) {
	a, err := Transform(budgetEnvelope("AWS Budgets: monthly-cost has exceeded your alert threshold", budgetMessage), Config{"severity": "major"})
	if err != nil {
		t.Fatal(err)
	}
	want := Alert{
		Service: "", MetricName: "AWS Budgets: monthly-cost", Severity: "major", Category: "service_interruption", Environment: "Unknown", Source: "AWS",
		UniqueIdentifier: "123456789012/monthly-cost",
		Description:      "You requested that we alert you when the ACTUAL Cost associated with your monthly-cost budget is greater than $80.00 for the current month. The ACTUAL Cost associated with this budget is $85.12. You can find additional details below and by accessing the AWS Budgets dashboard [1].",
		SourceTopic:      "arn:aws:sns:us-east-1:999988887777:budgets", SourceAccount: "123456789012",
	}
	if a != want {
		t.Errorf("alert =\n %+v\nwant\n %+v", a, want)
	}
}

// The actual and the forecast alert of one budget are one incident.
func TestTransform_BudgetForecastSharesTheIncident(t *testing.T) {
	actual, _ := Transform(budgetEnvelope("AWS Budgets: monthly-cost has exceeded your alert threshold", budgetMessage), Config{})
	forecast, _ := Transform(budgetEnvelope("AWS Budgets: monthly-cost is forecasted to exceed your alert threshold", budgetMessage), Config{})
	if actual.MetricName != forecast.MetricName || actual.UniqueIdentifier != forecast.UniqueIdentifier {
		t.Errorf("actual %q/%q, forecast %q/%q; want the same", actual.MetricName, actual.UniqueIdentifier, forecast.MetricName, forecast.UniqueIdentifier)
	}
}

// Without the Budget Name line the budget comes from the Subject and the account from the topic.
func TestTransform_BudgetNameFromSubject(t *testing.T) {
	a, err := Transform(budgetEnvelope("AWS Budgets: team-ec2 is forecasted to exceed your alert threshold", "Your budget is forecasted to exceed."), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if a.MetricName != "AWS Budgets: team-ec2" || a.UniqueIdentifier != "team-ec2" || a.SourceAccount != "999988887777" || a.Description != "Your budget is forecasted to exceed." {
		t.Errorf("alert = %+v", a)
	}
}

// Other plain-text messages keep the parse-error fallback.
func TestTransform_NonBudgetTextUnchanged(t *testing.T) {
	a, err := Transform(budgetEnvelope("Something else", "disk almost full"), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if a.MetricName != "SNS Message Parse Error" || a.Description != "disk almost full" || a.UniqueIdentifier != "" {
		t.Errorf("alert = %+v", a)
	}
}
