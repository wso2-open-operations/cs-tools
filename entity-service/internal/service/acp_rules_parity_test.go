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

package service

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// acpRulesParity is testdata/acp_rules_parity.json: the cases ACP's rules
// and this copy must agree on. ACP's own tests read the same file.
type acpRulesParity struct {
	DaysBetween []struct {
		Now  time.Time `json:"now"`
		End  time.Time `json:"end"`
		Want int       `json:"want"`
	} `json:"daysBetween"`
	OpportunityCounts []struct {
		Stage *string `json:"stage"`
		EULA  *string `json:"eula"`
		Want  bool    `json:"want"`
	} `json:"opportunityCounts"`
	EULAVersionDecimal []struct {
		EULA *string `json:"eula"`
		Want float64 `json:"want"`
	} `json:"eulaVersionDecimal"`
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
	InvoiceCounts []struct {
		Name           *string `json:"name"`
		Classification *string `json:"classification"`
		DueDate        *string `json:"dueDate"`
		PaidDate       *string `json:"paidDate"`
		ProjectStart   *string `json:"projectStart"`
		Want           bool    `json:"want"`
	} `json:"invoiceCounts"`
}

func TestACPRulesParity(t *testing.T) {
	raw, err := os.ReadFile("testdata/acp_rules_parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var c acpRulesParity
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	day := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatalf("fixture date %q: %v", s, err)
		}
		return d
	}
	for _, tc := range c.DaysBetween {
		if got := acpDaysBetween(tc.Now, tc.End); got != tc.Want {
			t.Errorf("daysBetween(%s, %s) = %d, want %d", tc.Now, tc.End, got, tc.Want)
		}
	}
	for _, tc := range c.OpportunityCounts {
		if got := acpOpportunityCounts(tc.Stage, tc.EULA); got != tc.Want {
			t.Errorf("opportunityCounts(%v, %v) = %v, want %v", derefString(tc.Stage), tc.EULA, got, tc.Want)
		}
	}
	for _, tc := range c.EULAVersionDecimal {
		if got := acpEULAVersionDecimal(tc.EULA); got != tc.Want {
			t.Errorf("eulaVersionDecimal(%v) = %v, want %v", tc.EULA, got, tc.Want)
		}
	}
	for _, tc := range c.UsesGracePeriod {
		if got := acpUsesGracePeriod(tc.EULA, tc.PrimaryPartner); got != tc.Want {
			t.Errorf("usesGracePeriod(%v, %v) = %v, want %v", tc.EULA, tc.PrimaryPartner, got, tc.Want)
		}
	}
	for _, tc := range c.InvoiceSuspendDate {
		got := acpInvoiceSuspendDate(day(tc.InvoiceDate), day(tc.DueDate), tc.EULA, tc.PrimaryPartner)
		if !got.Equal(day(tc.Want)) {
			t.Errorf("invoiceSuspendDate(%s, %s, %v, %v) = %s, want %s", tc.InvoiceDate, tc.DueDate, tc.EULA, tc.PrimaryPartner,
				got.Format(time.DateOnly), tc.Want)
		}
	}
	for i, tc := range c.InvoiceCounts {
		due, err1 := acpParseDate(tc.DueDate)
		paid, err2 := acpParseDate(tc.PaidDate)
		start, err3 := acpParseDate(tc.ProjectStart)
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatalf("invoiceCounts case %d: a fixture date does not parse", i)
		}
		inv := acpInvoice{Name: tc.Name, Classification: tc.Classification, DueDate: due, PaidDate: paid}
		if got := acpInvoiceCounts(inv, start); got != tc.Want {
			t.Errorf("invoiceCounts case %d = %v, want %v", i, got, tc.Want)
		}
	}
}
