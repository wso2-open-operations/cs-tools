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

package servicenow

import (
	"context"
	"testing"
)

func TestSanitizeQueryValue_Script(t *testing.T) {
	for _, ok := range []string{"jane.doe@example.com", "payment gateway", "CS0001234"} {
		if err := SanitizeQueryValue(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"a^ORb", "x\ny", "javascript:gs.getUser()", "JavaScript:1", "pre javascript: post"} {
		if err := SanitizeQueryValue(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRecordIDValidators(t *testing.T) {
	type check struct {
		fn   func(string) error
		name string
		ok   []string
		bad  []string
	}
	checks := []check{
		{ValidateRecordNumber, "ValidateRecordNumber",
			[]string{"CS0001234", "ACC12", "prj9"},
			[]string{"", "CS", "1234", "CS-0001", "CS0001^ORx", "0123456789abcdef0123456789abcdef", "javascript:1"}},
		{ValidateSysID, "ValidateSysID",
			[]string{"0123456789abcdef0123456789abcdef"},
			[]string{"", "0123456789ABCDEF0123456789ABCDEF", "0123456789abcdef0123456789abcde", "att-1", "CS0001234"}},
		{ValidateRecordNumberOrSysID, "ValidateRecordNumberOrSysID",
			[]string{"CS0001234", "0123456789abcdef0123456789abcdef"},
			[]string{"", "x^y", "javascript:1", "proj-1"}},
	}
	for _, c := range checks {
		for _, v := range c.ok {
			if err := c.fn(v); err != nil {
				t.Errorf("%s(%q) = %v, want nil", c.name, v, err)
			}
		}
		for _, v := range c.bad {
			if err := c.fn(v); err == nil {
				t.Errorf("%s(%q) = nil, want an error", c.name, v)
			}
		}
	}
}

func TestCaseLookupsRejectMalformedNumbersWithoutACall(t *testing.T) {
	c := &Client{} // a request would panic on the zero-value client
	if _, err := c.GetCaseByNumber(context.Background(), "CS1 OR 1=1"); err == nil {
		t.Error("GetCaseByNumber accepted a malformed number")
	}
	if _, err := c.GetEscalationsByAccount(context.Background(), "javascript:x", 0, 10); err == nil {
		t.Error("GetEscalationsByAccount accepted a malformed number")
	}
}
