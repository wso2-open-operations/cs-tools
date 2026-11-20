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

package service

import (
	"reflect"
	"testing"
)

func TestParseProductIssueFooter(t *testing.T) {
	tests := []struct {
		name, body string
		want       ProductIssueFooterInfo
	}{
		{"built footer", "Description.\n\nProduct : is-6.1.0\nUpdate level : U7\nWSO2 case id : ACME-1\nCase number : CS1\nPublic Git Issue : https://github.com/o/r/issues/9\nABT Team : Sirius\nOpened by : a@wso2.com",
			ProductIssueFooterInfo{OpenedBy: "a@wso2.com", ABTTeam: "Sirius", PublicIssueURL: "https://github.com/o/r/issues/9"}},
		{"undefined means none", "Public Git Issue : undefined\nABT Team : undefined\nOpened by : undefined", ProductIssueFooterInfo{}},
		{"last occurrence wins", "Opened by : first@wso2.com\nsome text\nOpened by : last@wso2.com\nABT Team : A\nABT Team : B", ProductIssueFooterInfo{OpenedBy: "last@wso2.com", ABTTeam: "B"}},
		{"a last undefined wins over an earlier value", "Opened by : first@wso2.com\nOpened by : undefined", ProductIssueFooterInfo{}},
		{"value with a colon inside", "Public Git Issue : https://x.test/a : b\nABT Team : Team : Alpha", ProductIssueFooterInfo{PublicIssueURL: "https://x.test/a : b", ABTTeam: "Team : Alpha"}},
		{"CRLF body", "text\r\n\r\nABT Team : Sirius\r\nOpened by : a@wso2.com\r\n", ProductIssueFooterInfo{OpenedBy: "a@wso2.com", ABTTeam: "Sirius"}},
		{"no footer", "Just a description : with a colon-ish thing\nNothing else.", ProductIssueFooterInfo{}},
		{"empty body", "", ProductIssueFooterInfo{}},
		{"label case and padding", "  opened BY  : a@wso2.com  ", ProductIssueFooterInfo{OpenedBy: "a@wso2.com"}},
		{"colon without spaces is not a footer line", "Opened by: a@wso2.com", ProductIssueFooterInfo{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseProductIssueFooter(tc.body); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The parser reads what the builder writes.
func TestParseProductIssueFooter_RoundTrip(t *testing.T) {
	body := buildProductIssueBody(pwReq(), "product/is", productIssueFooterContext{Number: "CS1", InternalID: "ACME-1", Team: "Sirius", OpenedBy: pwEmail})
	got := ParseProductIssueFooter(body)
	if got.OpenedBy != pwEmail || got.ABTTeam != "Sirius" || got.PublicIssueURL != "" {
		t.Errorf("got %+v", got)
	}
}

func TestIssueLabelHelpers(t *testing.T) {
	labels := []string{"Origin/CS", "product/is", "Type/Incident", "Priority/High(P2)"}
	if got := issuePriorityFromLabels(labels); got != "P2" {
		t.Errorf("priority = %q", got)
	}
	if got := issuePriorityFromLabels([]string{"Priority/Critical", "x"}); got != "P1" {
		t.Errorf("priority = %q", got)
	}
	if got := issuePriorityFromLabels([]string{"Origin/CS"}); got != "" {
		t.Errorf("priority = %q", got)
	}
	if got := issueCategoryFromLabels(labels); got != "Type/Incident" {
		t.Errorf("category = %q", got)
	}
	if !isPatchIssue([]string{"Type/Patch"}) || !isPatchIssue([]string{"Patch"}) || isPatchIssue(labels) {
		t.Error("isPatchIssue")
	}
}
