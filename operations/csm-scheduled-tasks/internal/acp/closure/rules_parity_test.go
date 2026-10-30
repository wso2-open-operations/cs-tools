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

package closure

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// parityFile is the file of cases ACP's rules and entity-service's copy of
// them (the ACP reopen check, acp_reopen_rules.go) must agree on. A
// disagreement makes ACP suspend a project every day and the reopen check
// reopen it. ACP_RULES_PARITY_FILE overrides the path; without the file (a
// checkout of this component alone) the test is skipped.
func parityFile(t *testing.T) []byte {
	t.Helper()
	path := os.Getenv("ACP_RULES_PARITY_FILE")
	if path == "" {
		path = "../../../../../entity-service/internal/service/testdata/acp_rules_parity.json"
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("%s not found; the rules parity cases live in entity-service", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestRulesParity runs the closure package's share of the parity cases.
func TestRulesParity(t *testing.T) {
	var c struct {
		DaysBetween []struct {
			Now  time.Time `json:"now"`
			End  time.Time `json:"end"`
			Want int       `json:"want"`
		} `json:"daysBetween"`
		UsesGracePeriod []struct {
			EULA           float64 `json:"eula"`
			PrimaryPartner bool    `json:"primaryPartner"`
			Want           bool    `json:"want"`
		} `json:"usesGracePeriod"`
		InvoiceSuspendDate []struct {
			InvoiceDate    string  `json:"invoiceDate"`
			DueDate        string  `json:"dueDate"`
			EULA           float64 `json:"eula"`
			PrimaryPartner bool    `json:"primaryPartner"`
			Want           string  `json:"want"`
		} `json:"invoiceSuspendDate"`
	}
	if err := json.Unmarshal(parityFile(t), &c); err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatalf("fixture date %q: %v", s, err)
		}
		return d
	}
	if len(c.DaysBetween) == 0 || len(c.UsesGracePeriod) == 0 || len(c.InvoiceSuspendDate) == 0 {
		t.Fatal("the parity file has no cases for the closure rules")
	}
	for _, tc := range c.DaysBetween {
		if got := daysBetween(tc.Now, tc.End); got != tc.Want {
			t.Errorf("daysBetween(%s, %s) = %d, want %d", tc.Now, tc.End, got, tc.Want)
		}
	}
	for _, tc := range c.UsesGracePeriod {
		if got := usesGracePeriod(tc.EULA, tc.PrimaryPartner); got != tc.Want {
			t.Errorf("usesGracePeriod(%v, %v) = %v, want %v", tc.EULA, tc.PrimaryPartner, got, tc.Want)
		}
	}
	for _, tc := range c.InvoiceSuspendDate {
		got := InvoiceSuspendDate(day(tc.InvoiceDate), day(tc.DueDate), tc.EULA, tc.PrimaryPartner)
		if !got.Equal(day(tc.Want)) {
			t.Errorf("InvoiceSuspendDate(%s, %s, %v, %v) = %s, want %s", tc.InvoiceDate, tc.DueDate, tc.EULA, tc.PrimaryPartner,
				got.Format(time.DateOnly), tc.Want)
		}
	}
}
