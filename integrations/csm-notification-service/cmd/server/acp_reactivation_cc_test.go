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
	"strings"
	"testing"
)

func TestParseACPReactivationCC(t *testing.T) {
	got, err := parseACPReactivationCC(" lifecycle@example.com , ,billing@example.com")
	if err != nil || len(got) != 2 || got[0] != "lifecycle@example.com" || got[1] != "billing@example.com" {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := parseACPReactivationCC(""); err != nil || got != nil {
		t.Errorf("empty: got %v, %v", got, err)
	}
	if _, err := parseACPReactivationCC("lifecycle@example.com,billing@example"); err == nil {
		t.Error("a malformed entry was accepted")
	} else if strings.Contains(err.Error(), "billing") {
		t.Errorf("the error names the address: %v", err)
	}
}
