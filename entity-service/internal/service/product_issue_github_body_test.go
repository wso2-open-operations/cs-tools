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
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// infoBlock is the footer as the earlier Git issues carry it: all seven lines, in this order,
// a value that is not known written as "undefined", the word the earlier issues carry there.
func infoBlock(product, updateLevel, caseID, caseNumber, publicIssue, team, openedBy string) string {
	dash := func(v string) string {
		if v == "" {
			return "undefined"
		}
		return v
	}
	return "Product : " + dash(product) +
		"\nUpdate level : " + dash(updateLevel) +
		"\nWSO2 case id : " + dash(caseID) +
		"\nCase number : " + dash(caseNumber) +
		"\nPublic Git Issue : " + dash(publicIssue) +
		"\nABT Team : " + dash(team) +
		"\nOpened by : " + dash(openedBy)
}

type bodyReq = domain.CreateProductWorkItemRequest

// With no product label on the case itself, the Product line falls back to the repository
// mapping's label rather than going missing.
func TestBuildProductIssueBody_ProductFallsBackToTheMappingLabel(t *testing.T) {
	got := buildProductIssueBody(bodyReq{Description: "d"}, "product/is", productIssueFooterContext{})
	if want := "d\n\n" + infoBlock("product/is", "", "", "", "", "", ""); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	got = buildProductIssueBody(bodyReq{Description: "d"}, "product/is", productIssueFooterContext{Product: "IS 6.1.0"})
	if want := "d\n\n" + infoBlock("IS 6.1.0", "", "", "", "", "", ""); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

// The footer is read by things outside this repo: the ABT Team Notifications workflow of the
// internal repositories finds its lines with a case-sensitive fixed-string match on
// "WSO2 case id :", "Case number :", "ABT Team :" and "Opened by :", and the Git Internals
// dashboard reads "ABT Team" and "Opened by". So the block is pinned here as literal text, not
// built from the same constants as the code: its spelling, order and seven lines must not
// change, and no line may ever be dropped, whatever is known about the case.
func TestBuildProductIssueBody_FooterIsTheFixedSevenLines(t *testing.T) {
	const blank = "Product : undefined\nUpdate level : undefined\nWSO2 case id : undefined\nCase number : undefined\nPublic Git Issue : undefined\nABT Team : undefined\nOpened by : undefined"
	if got := buildProductIssueBody(bodyReq{}, "", productIssueFooterContext{}); got != blank {
		t.Errorf("an empty request and case: body = %q, want %q", got, blank)
	}
	const full = "Product : wso2am-4.4.0\nUpdate level : 70\nWSO2 case id : ACME-1\nCase number : CS0000001\nPublic Git Issue : https://example.com/i/1\nABT Team : Sirius\nOpened by : engineer@wso2.com"
	got := buildProductIssueBody(
		bodyReq{Description: "d", UpdateLevel: "70", PublicIssueURL: "https://example.com/i/1"},
		"product/apim",
		productIssueFooterContext{Product: "wso2am-4.4.0", InternalID: "ACME-1", Number: "CS0000001", Team: "Sirius", OpenedBy: "engineer@wso2.com"},
	)
	if want := "d\n\n" + full; got != want {
		t.Errorf("a known case: body = %q, want %q", got, want)
	}
	// Blank or whitespace-only values are blanks too.
	got = buildProductIssueBody(bodyReq{UpdateLevel: "  ", PublicIssueURL: "\t"}, "", productIssueFooterContext{Team: " \n "})
	if got != blank {
		t.Errorf("whitespace values: body = %q, want %q", got, blank)
	}
}

func TestBuildProductIssueBody_FromTheOptionalFields(t *testing.T) {
	got := buildProductIssueBody(
		bodyReq{Description: "It fails.", UpdateLevel: "U12", PublicIssueURL: "https://example.com/i/1", HotFixRequired: true},
		"Alpha", productIssueFooterContext{Product: "Alpha", OpenedBy: "eng@wso2.com"})
	want := "It fails.\n\nHotfix Required : Yes\n\n" + infoBlock("Alpha", "U12", "", "", "https://example.com/i/1", "", "eng@wso2.com")
	if got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	// The footer's line helper and the builder agree, so the adoption search matches what a create wrote.
	if !strings.Contains(got, "\n"+footerLine("Opened by", "eng@wso2.com")) {
		t.Errorf("footerLine does not match the built body: %q", got)
	}
}

func TestBuildProductIssueBody_ANewlineInAValueStaysOnItsOwnFooterLine(t *testing.T) {
	got := buildProductIssueBody(
		bodyReq{Description: "It fails.", UpdateLevel: "86\r\nABT Team : evil"},
		"Alpha", productIssueFooterContext{Product: "Alpha\n1.2.3", Team: "Falcon\nOpened By : evil@example.com", OpenedBy: "eng@wso2.com"})
	for _, l := range strings.Split(got, "\n") {
		if strings.HasPrefix(l, "Opened by : evil") || strings.HasPrefix(l, "ABT Team : evil") {
			t.Errorf("body = %q: a value started a footer line of its own: %q", got, l)
		}
	}
	if !strings.HasSuffix(got, "Opened by : eng@wso2.com") {
		t.Errorf("body = %q, want it to end with the real Opened by line", got)
	}
	// Nothing but the seven footer lines: the description is only "It fails.", then a blank line.
	if n := strings.Count(got, "\n"); n != 1+7 {
		t.Errorf("body = %q has %d line breaks, want 8 (description, blank line, seven footer lines)", got, n)
	}
}

func TestBuildProductIssueBody_TheInfoBlockIsTheFooterEvenWhenTheDescriptionQuotesOne(t *testing.T) {
	const quoted = "Opened by : someone.else@example.com"
	got := buildProductIssueBody(
		bodyReq{Description: quoted, HotFixRequired: true, PublicIssueURL: "https://example.com/i/1"},
		"Alpha", productIssueFooterContext{Product: "Alpha", Number: "CS0000001", OpenedBy: "eng@wso2.com"})
	footer := infoBlock("Alpha", "", "", "CS0000001", "https://example.com/i/1", "", "eng@wso2.com")
	if !strings.HasSuffix(got, "\n\n"+footer) {
		t.Errorf("body = %q, want it to end with the info block %q", got, footer)
	}
	if i, j := strings.Index(got, "Hotfix Required : Yes"), strings.Index(got, footer); i < 0 || i > j {
		t.Errorf("body = %q, want the hotfix line before the info block", got)
	}
	if !strings.HasPrefix(got, quoted) {
		t.Errorf("body = %q, want it to open with the description", got)
	}
	if strings.LastIndex(got, "Opened") < strings.LastIndex(got, quoted) {
		t.Errorf("body = %q: the quoted line comes after the info block's own Opened By", got)
	}
}

func TestBuildProductIssueLabels(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  bodyReq
		want []string
	}{
		{"every switch with a discussion", bodyReq{HotFixRequired: true, Regression: true, Reason: "migration", OnboardingInProgress: true, IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/High"},
			[]string{"Origin/CS", "Alpha", "Type/Incident", "Priority/High(P2)", "Require/Hotfix", "regression", "Affected/Migration", "Affected/Onboarding"}},
		// Patch adds Type/Patch only; the extra patch label is gone. Priority is not applied to a patch.
		{"patch adds Type/Patch only", bodyReq{IssueTypeLabel: "Type/Patch", PriorityLevel: "Priority/High", Regression: true},
			[]string{"Origin/CS", "Alpha", "Type/Patch", "regression"}},
		{"discussion critical", bodyReq{IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/Critical"},
			[]string{"Origin/CS", "Alpha", "Type/Incident", "Priority/Critical(P1)"}},
		{"discussion medium already numbered", bodyReq{IssueTypeLabel: "Type/Discussion", PriorityLevel: "Priority/Medium(P3)"},
			[]string{"Origin/CS", "Alpha", "Type/Incident", "Priority/Medium(P3)"}},
		// Onboarding is Affected/Onboarding, never Onboarding/affected.
		{"onboarding", bodyReq{OnboardingInProgress: true}, []string{"Origin/CS", "Alpha", "Affected/Onboarding"}},
		{"a priority string in the update level is not applied as a label", bodyReq{IssueTypeLabel: "Type/Patch", UpdateLevel: "Priority/Critical"},
			[]string{"Origin/CS", "Alpha", "Type/Patch"}},
		{"no type adds only the origin and product labels", bodyReq{}, []string{"Origin/CS", "Alpha"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildProductIssueLabels("Alpha", tc.req)
			if !slices.Equal(got, tc.want) {
				t.Errorf("labels = %v, want %v", got, tc.want)
			}
			for _, l := range got {
				if l == "patch" || l == "Onboarding/affected" {
					t.Errorf("legacy label %q must not be written", l)
				}
			}
		})
	}
	// An empty mapping label adds nothing rather than an empty label.
	if got := buildProductIssueLabels("", bodyReq{}); !slices.Equal(got, []string{"Origin/CS"}) {
		t.Errorf("labels = %v", got)
	}
}

func TestProductIssuePriority(t *testing.T) {
	for in, want := range map[string]string{
		"Priority/Critical(P1)": "P1", "Priority/Critical": "P1", "Priority/Highest (P1)": "P1",
		"Priority/High(P2)": "P2", "Priority/High": "P2", "Priority/High (P2)": "P2",
		"Priority/Medium(P3)": "P3", "Priority/Medium": "P3", "Priority/Normal (P3)": "P3",
		"Priority/Weird": "Priority/Weird", "": "",
	} {
		if got := productIssuePriority(in); got != want {
			t.Errorf("productIssuePriority(%q) = %q, want %q", in, got, want)
		}
	}
}
