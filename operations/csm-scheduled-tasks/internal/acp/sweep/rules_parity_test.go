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

package sweep

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// parityFile is the file of cases ACP's rules and entity-service's copy of
// them (the ACP reopen check) must agree on; see closure's parityFile.
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

// TestRulesParity runs the sweep package's share of the parity cases: which
// opportunities and invoices count, and the EULA version number.
func TestRulesParity(t *testing.T) {
	var c struct {
		OpportunityCounts []struct {
			Stage *string `json:"stage"`
			EULA  *string `json:"eula"`
			Want  bool    `json:"want"`
		} `json:"opportunityCounts"`
		EULAVersionDecimal []struct {
			EULA *string `json:"eula"`
			Want float64 `json:"want"`
		} `json:"eulaVersionDecimal"`
		InvoiceCounts []struct {
			Name           *string `json:"name"`
			Classification *string `json:"classification"`
			DueDate        *string `json:"dueDate"`
			PaidDate       *string `json:"paidDate"`
			ProjectStart   *string `json:"projectStart"`
			Want           bool    `json:"want"`
		} `json:"invoiceCounts"`
	}
	if err := json.Unmarshal(parityFile(t), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.OpportunityCounts) == 0 || len(c.EULAVersionDecimal) == 0 || len(c.InvoiceCounts) == 0 {
		t.Fatal("the parity file has no cases for the sweep rules")
	}
	for _, tc := range c.OpportunityCounts {
		if got := eligibleOpportunity(opportunityDTO{Stage: tc.Stage, EulaVersion: tc.EULA}); got != tc.Want {
			t.Errorf("eligibleOpportunity(%v, %v) = %v, want %v", stringValue(tc.Stage), tc.EULA, got, tc.Want)
		}
	}
	for _, tc := range c.EULAVersionDecimal {
		got, err := parseEULAVersionDecimal(eulaVersionDecimal(tc.EULA))
		if err != nil {
			t.Errorf("EULA %v: %v", tc.EULA, err)
			continue
		}
		if got != tc.Want {
			t.Errorf("EULA version of %v = %v, want %v", tc.EULA, got, tc.Want)
		}
	}
	for i, tc := range c.InvoiceCounts {
		var start *time.Time
		if tc.ProjectStart != nil {
			d, err := time.Parse(time.DateOnly, *tc.ProjectStart)
			if err != nil {
				t.Fatalf("case %d: project start: %v", i, err)
			}
			start = &d
		}
		inv := invoiceDTO{Name: tc.Name, Classification: tc.Classification, InvoicedDueDate: tc.DueDate, InvoicedPaidDate: tc.PaidDate}
		if got := eligibleInvoice(inv, start); got != tc.Want {
			t.Errorf("eligibleInvoice case %d = %v, want %v", i, got, tc.Want)
		}
	}
}
