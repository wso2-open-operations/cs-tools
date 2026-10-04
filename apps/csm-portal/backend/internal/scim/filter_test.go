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

package scim

import (
	"context"
	"testing"
)

func TestUserNameEqFilter(t *testing.T) {
	cases := map[string]string{
		"jane.doe@example.com": `userName eq "jane.doe@example.com"`,
		`a"b@example.com`:      `userName eq "a\"b@example.com"`,
		`a\b@example.com`:      `userName eq "a\\b@example.com"`,
		"x or userName sw j":   `userName eq "x or userName sw j"`,
	}
	for in, want := range cases {
		if got := userNameEqFilter(in); got != want {
			t.Errorf("userNameEqFilter(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestValidateFilterEmail(t *testing.T) {
	for _, ok := range []string{"jane.doe@example.com", "a+b@sub.example.co"} {
		if err := validateFilterEmail(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "x or userName sw j", "jane@", "@example.com", "a@b@c", `a"b@example.com`, `a\b@example.com`, "a\tb@example.com", "noatsign"} {
		if err := validateFilterEmail(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSearchRejectsInvalidEmailWithoutACall(t *testing.T) {
	c := &Client{} // a call would panic on the nil http client
	if _, err := c.SearchUser(context.Background(), "x or userName sw j"); err == nil {
		t.Error("SearchUser accepted a non-email value")
	}
	if _, err := c.SearchExternalUser(context.Background(), "x or userName sw j"); err == nil {
		t.Error("SearchExternalUser accepted a non-email value")
	}
}
