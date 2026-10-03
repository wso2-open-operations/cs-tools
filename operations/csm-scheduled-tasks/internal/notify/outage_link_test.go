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
package notify

import (
	"strings"
	"testing"
)

func TestOutageLink(t *testing.T) {
	cases := []struct{ base, id, want string }{
		{"https://csm-stg.apps.wso2.com", "3ca9cf3d-566a-4345-93b2-86845772be45",
			"https://csm-stg.apps.wso2.com/operations/outages/3ca9cf3d-566a-4345-93b2-86845772be45"},
		{"https://csm-stg.apps.wso2.com/", "abc", "https://csm-stg.apps.wso2.com/operations/outages/abc"},
		{"", "abc", ""}, // unset base: no link, not a relative one
		{"https://csm-stg.apps.wso2.com", "", ""}, // no id: no link to the list page
		{"https://x.example", "a/b", "https://x.example/operations/outages/a%2Fb"},
	}
	for _, c := range cases {
		if got := OutageLink(c.base, c.id); got != c.want {
			t.Errorf("OutageLink(%q, %q) = %q, want %q", c.base, c.id, got, c.want)
		}
	}
}

func TestRenderOutageNotificationLink(t *testing.T) {
	data := OutageNotificationData{PhaseWord: "Declared", Number: "OUT0010000", Message: "Outage OUT0010000 declared.",
		Link: "https://csm-stg.apps.wso2.com/operations/outages/3ca9cf3d"}
	body := RenderOutageNotification(data)
	if !strings.Contains(body, `href="https://csm-stg.apps.wso2.com/operations/outages/3ca9cf3d"`) ||
		!strings.Contains(body, "View outage in the CSM portal") {
		t.Fatalf("notification is missing the outage link:\n%s", body)
	}
	if strings.Contains(body, "<!-- [OUTAGE_LINK] -->") {
		t.Fatal("placeholder left unreplaced")
	}

	data.Link = ""
	if body := RenderOutageNotification(data); strings.Contains(body, "View outage") || strings.Contains(body, "[OUTAGE_LINK]") {
		t.Fatalf("no link configured, but the email still carries one or its placeholder:\n%s", body)
	}

	data.Link = `https://x.example/"><script>`
	if body := RenderOutageNotification(data); strings.Contains(body, `"><script>`) {
		t.Fatal("link is not attribute-escaped")
	}
}

func TestCheckPortalBaseURL(t *testing.T) {
	for _, ok := range []string{
		"https://csm-stg.apps.wso2.com",
		"https://csm-stg.apps.wso2.com/",
		"https://host.example/portal",
		"http://localhost:5173", // loopback exempt, as everywhere in httpsec
	} {
		if err := CheckPortalBaseURL(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"http://csm-stg.apps.wso2.com",       // not https
		"https://csm-stg.apps.wso2.com/?x=1", // query lands before the route
		"https://csm-stg.apps.wso2.com/?",    // empty query is still a '?'
		"https://csm-stg.apps.wso2.com/#top", // fragment swallows the route
		"https://csm-stg.apps.wso2.com/#",
		"https://user:pw@csm-stg.apps.wso2.com", // credentials in a mailed link
	} {
		if err := CheckPortalBaseURL(bad); err == nil {
			t.Errorf("%q accepted, want rejected", bad)
		}
	}
}
