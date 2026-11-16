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

package main

import (
	"reflect"
	"testing"
)

func TestParseACPInternalEmailDomains(t *testing.T) {
	got, err := parseACPInternalEmailDomains(" @example.com , ,example.org")
	if err != nil || !reflect.DeepEqual(got, []string{"@example.com", "example.org"}) {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := parseACPInternalEmailDomains(""); err != nil || got != nil {
		t.Errorf("empty: got %v, %v", got, err)
	}
	for _, bad := range []string{"@localhost", "me@example.com", "@.com", "@example .com"} {
		if _, err := parseACPInternalEmailDomains(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
