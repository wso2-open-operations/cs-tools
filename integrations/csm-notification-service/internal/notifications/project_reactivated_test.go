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

package notifications

import (
	"strings"
	"testing"
)

func TestRenderProjectReactivatedEmail_EscapesEveryValue(t *testing.T) {
	body := RenderProjectReactivatedEmail(ProjectReactivatedEmailData{
		GreetingName: `<b>Alex</b>`,
		ReasonLine:   "Reopened & ready",
		ProjectName:  `Acme <script>alert(1)</script>`,
		ProjectKey:   "KEY",
		ProjectLink:  `https://csm.example/customers/projects/1?a=1&b="2"`,
		AccountOwner: "Alex Ówner",
		StartDate:    "2026-10-28",
		EndDate:      "2027-10-27",
	})
	for _, bad := range []string{"<script>", "<b>Alex</b>", `b="2"`, "Ó"} {
		if strings.Contains(body, bad) {
			t.Errorf("body carries %q unescaped", bad)
		}
	}
	for _, want := range []string{"&lt;script&gt;", "Reopened &amp; ready", "&amp;b=&#34;2&#34;", "&#211;wner", "<!DOCTYPE html>", wso2LogoURL} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "<!-- [") {
		t.Error("a placeholder was left in the body")
	}
}

func TestRenderProjectReactivatedEmail_EmptyValuesShowADash(t *testing.T) {
	body := RenderProjectReactivatedEmail(ProjectReactivatedEmailData{GreetingName: "Team", ReasonLine: "x", ProjectName: "P"})
	for _, want := range []string{"Project Key: <b>-</b>", "Account Owner: <b>-</b>", "Start Date: <b>-</b>", "End Date: <b>-</b>"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "Expected sent to") {
		t.Error("the debug row shows on a real send")
	}
}
