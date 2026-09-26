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

package ingest

import (
	"strings"
	"testing"
)

// TestExtractIssueMeta covers the key/value lines ExtractIssueMeta must
// recognize, the placeholder and domain values it must reject, and the
// markdown/whitespace noise it must tolerate around a key.
func TestExtractIssueMeta(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantABTTeam  *string
		wantOpenedBy *string
	}{
		{
			name: "real world example body",
			body: strings.Join([]string{
				"Product : wso2is-5.11.0",
				"Update level : undefined",
				"WSO2 case id : EXAMPLECASE-001",
				"Case number : CS0000001",
				"Public Git Issue : undefined",
				"ABT Team : Atlas",
				"Opened by : xxx@wso2.com",
			}, "\n"),
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: strp("xxx@wso2.com"),
		},
		{
			name:         "body with neither key",
			body:         "Just some free-text description of the problem.",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "empty body",
			body:         "",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "placeholder values",
			body:         "ABT Team : undefined\nOpened by : undefined",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "bold key with no space before colon",
			body:         "**ABT Team:** Atlas",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: nil,
		},
		{
			name:         "bullet prefixed key",
			body:         "- ABT Team: Atlas",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: nil,
		},
		{
			name:         "blockquote prefixed key with extra internal spaces",
			body:         ">   ABT   Team   :   Atlas",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: nil,
		},
		{
			name:         "lower case key spelling",
			body:         "abt team: Atlas",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: nil,
		},
		{
			name:         "upper case key spelling",
			body:         "ABT TEAM: Atlas",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: nil,
		},
		{
			name:         "crlf line endings",
			body:         "ABT Team : Atlas\r\nOpened by : xxx@wso2.com\r\n",
			wantABTTeam:  strp("Atlas"),
			wantOpenedBy: strp("xxx@wso2.com"),
		},
		{
			name:         "mixed case email is lower-cased",
			body:         "Opened by : Jane.Doe@WSO2.com",
			wantABTTeam:  nil,
			wantOpenedBy: strp("jane.doe@wso2.com"),
		},
		{
			name:         "markdown mailto link",
			body:         "Opened by : [jane@wso2.com](mailto:jane@wso2.com)",
			wantABTTeam:  nil,
			wantOpenedBy: strp("jane@wso2.com"),
		},
		{
			name:         "angle bracket wrapped address",
			body:         "Opened by : <jane@wso2.com>",
			wantABTTeam:  nil,
			wantOpenedBy: strp("jane@wso2.com"),
		},
		{
			name:         "non wso2 domain is rejected",
			body:         "Opened by : someone@gmail.com",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "look-alike domain suffix is rejected",
			body:         "Opened by : someone@wso2.com.evil.io",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "subdomain is rejected",
			body:         "Opened by : someone@sub.wso2.com",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "repeated key last occurrence wins",
			body:         "ABT Team : Atlas\nABT Team : Voyager",
			wantABTTeam:  strp("Voyager"),
			wantOpenedBy: nil,
		},
		{
			name:         "abt team longer than max length is rejected",
			body:         "ABT Team : " + strings.Repeat("a", 101),
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "abt team at max length is accepted",
			body:         "ABT Team : " + strings.Repeat("a", 100),
			wantABTTeam:  strp(strings.Repeat("a", 100)),
			wantOpenedBy: nil,
		},
		{
			name:         "extra trailing word after key does not match",
			body:         "Opened byx: someone@wso2.com",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
		{
			name:         "extra leading word before key does not match",
			body:         "Previously opened by: someone@wso2.com",
			wantABTTeam:  nil,
			wantOpenedBy: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractIssueMeta(tc.body)
			if !strPtrEqual(got.ABTTeam, tc.wantABTTeam) {
				t.Errorf("ABTTeam = %s, want %s", strPtrDisplay(got.ABTTeam), strPtrDisplay(tc.wantABTTeam))
			}
			if !strPtrEqual(got.OpenedBy, tc.wantOpenedBy) {
				t.Errorf("OpenedBy = %s, want %s", strPtrDisplay(got.OpenedBy), strPtrDisplay(tc.wantOpenedBy))
			}
		})
	}
}

// TestNormalizeTitle verifies whitespace is trimmed and an empty or
// all-whitespace title becomes nil rather than an empty string.
func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *string
	}{
		{"leading and trailing whitespace trimmed", "  Some Title  ", strp("Some Title")},
		{"empty string yields nil", "", nil},
		{"all-whitespace string yields nil", "   \t  ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTitle(tc.in)
			if !strPtrEqual(got, tc.want) {
				t.Errorf("normalizeTitle(%q) = %s, want %s", tc.in, strPtrDisplay(got), strPtrDisplay(tc.want))
			}
		})
	}
}

// strPtrEqual reports whether two *string values represent the same
// presence/absence and, when both present, the same underlying value.
func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// strPtrDisplay renders a *string for a test failure message, showing
// "<nil>" for an absent value instead of dereferencing a nil pointer.
func strPtrDisplay(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
